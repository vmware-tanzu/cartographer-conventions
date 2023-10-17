/*
Copyright 2020 VMware Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package binding

import (
	"archive/tar"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"

	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

type RegistryConfig struct {
	Keys       authn.Keychain
	Cache      cache.Cache
	Client     kubernetes.Interface
	CACertPath string
}

type imageError map[string]error

func (e imageError) Error() string {
	var aggregatedError string
	for imageName, err := range e {
		// TODO(shashwathi): best way to aggregate error msgs for all images. tab or new line
		aggregatedError = aggregatedError + fmt.Sprintf("image: %q error: %s", imageName, err)
	}
	return aggregatedError
}

func getImagesSet(template *corev1.PodTemplateSpec) sets.String {
	images := sets.NewString()
	for _, container := range template.Spec.InitContainers {
		images.Insert(container.Image)
	}
	for _, container := range template.Spec.Containers {
		images.Insert(container.Image)
	}
	return images
}

func updateWithResolvedDigest(template *corev1.PodTemplateSpec, imageDigest map[string]string) {
	for i := range template.Spec.InitContainers {
		c := &template.Spec.InitContainers[i]
		if digest, ok := imageDigest[c.Image]; ok {
			c.Image = digest
		}
		template.Spec.InitContainers[i] = *c
	}
	for i := range template.Spec.Containers {
		c := &template.Spec.Containers[i]
		if digest, ok := imageDigest[c.Image]; ok {
			c.Image = digest
		}
		template.Spec.Containers[i] = *c
	}
}

func (rc *RegistryConfig) ResolveImageMetadata(ctx context.Context, template *corev1.PodTemplateSpec) ([]webhookv1alpha1.ImageConfig, error) {
	if template == nil {
		return nil, nil
	}

	images := getImagesSet(template)
	imageDigest := make(map[string]string)
	var imageConfigList []webhookv1alpha1.ImageConfig
	var imageErrMap = map[string]error{}
	for _, image := range images.List() {
		if image != "" {
			imageConfig, err := rc.resolveImageMetadata(ctx, image, name.WeakValidation)
			if err != nil {
				imageErrMap[image] = err
				continue
			}
			imageConfigList = append(imageConfigList, imageConfig)
			imageDigest[image] = imageConfig.Image
		}
	}
	if len(imageErrMap) > 0 {
		return imageConfigList, imageError(imageErrMap)
	}
	// update workload with resolved the image references.
	updateWithResolvedDigest(template, imageDigest)
	return imageConfigList, nil
}

func (rc *RegistryConfig) resolveImageMetadata(ctx context.Context, imageRef string, opts ...name.Option) (webhookv1alpha1.ImageConfig, error) {
	ref, resolved, err := resolveTagsToDigest(imageRef, opts...)
	if err != nil {
		return webhookv1alpha1.ImageConfig{}, fmt.Errorf("failed to resolve image %q: %v as digest could not be determined from tag provided", imageRef, err)
	}
	if rc.Keys == nil {
		return webhookv1alpha1.ImageConfig{}, fmt.Errorf("registry config keys are not set")
	}

	rt, err := rc.transport()
	if err != nil {
		return webhookv1alpha1.ImageConfig{}, err
	}

	image, err := remote.Image(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(rc.Keys), remote.WithTransport(rt))
	if err != nil {
		return webhookv1alpha1.ImageConfig{}, err
	}
	if rc.Cache != nil {
		image = cache.Image(image, rc.Cache)
	}
	config, err := image.ConfigFile()
	if err != nil {
		return webhookv1alpha1.ImageConfig{}, err
	}

	// sbom lookup deeply inspired by https://github.com/sclevine/cnb-sbom/blob/571237ed5e63ade40f0ccf4d8467fa5abd3f8872/main.go#L148-L190
	appDiffId, err := rc.resolveSBOMDiffId(config)
	if err != nil {
		return webhookv1alpha1.ImageConfig{}, err
	}
	var sboms []webhookv1alpha1.BOM
	if appDiffId != "" {
		sboms, err = rc.loadSBOMs(image, appDiffId, "cnb-app")
		if err != nil {
			return webhookv1alpha1.ImageConfig{}, err
		}
	}

	imageName := ref.Name()
	if !resolved {
		dig, err := image.Digest()
		if err != nil {
			return webhookv1alpha1.ImageConfig{}, err
		}
		imageName = fmt.Sprintf("%s@%s", ref.Name(), dig.String())
	}

	return webhookv1alpha1.ImageConfig{
		Image:  imageName,
		BOMs:   sboms,
		Config: *config,
	}, nil
}

// transport with systemCA and customCAs (if defined)
func (rc *RegistryConfig) transport() (http.RoundTripper, error) {
	if rc.CACertPath == "" {
		return remote.DefaultTransport, nil
	}

	transport := remote.DefaultTransport.(*http.Transport).Clone()
	// seed with system cert pool
	if root, err := x509.SystemCertPool(); err != nil {
		return transport, err
	} else {
		transport.TLSClientConfig.RootCAs = root
	}
	// append additional ca
	additionalCA, err := os.ReadFile(rc.CACertPath)
	if err != nil {
		return nil, err
	}

	transport.TLSClientConfig.RootCAs.AppendCertsFromPEM(additionalCA)
	return transport, nil
}

func (rc *RegistryConfig) resolveSBOMDiffId(config *v1.ConfigFile) (string, error) {
	if config.Config.Labels == nil {
		return "", nil
	}

	diffID := config.Config.Labels["io.buildpacks.app.sbom"]
	if diffID == "" {
		// fallback if the shortcut label is not set
		var md struct {
			SBOM struct {
				SHA string
			}
			BOM struct {
				SHA string
			}
		}
		metadata := config.Config.Labels["io.buildpacks.lifecycle.metadata"]
		if metadata == "" {
			return "", nil
		}
		if err := json.Unmarshal([]byte(metadata), &md); err != nil {
			return "", err
		}
		if diffID = md.SBOM.SHA; diffID == "" {
			diffID = md.BOM.SHA
		}
	}

	return diffID, nil
}

func (rc *RegistryConfig) loadSBOMs(image v1.Image, diffID string, prefix string) ([]webhookv1alpha1.BOM, error) {
	hash, err := v1.NewHash(diffID)
	if err != nil {
		return nil, err
	}
	layer, err := image.LayerByDiffID(hash)
	if err != nil {
		return nil, err
	}
	tar, err := layer.Uncompressed()
	if err != nil {
		return nil, err
	}
	defer tar.Close()
	return rc.untarSBOMs(tar, prefix)
}

func (rc *RegistryConfig) untarSBOMs(r io.Reader, prefix string) ([]webhookv1alpha1.BOM, error) {
	boms := []webhookv1alpha1.BOM{}
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		raw, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		boms = append(boms, webhookv1alpha1.BOM{
			Name: fmt.Sprintf("%s:%s", prefix, header.Name),
			Raw:  raw,
		})
	}
	return boms, nil
}

func resolveTagsToDigest(imageRef string, opts ...name.Option) (name.Reference, bool, error) {
	digest, derr := name.NewDigest(imageRef, opts...)
	if derr == nil {
		//already resolved to digest
		return digest, true, nil
	}
	tag, err := name.NewTag(imageRef, opts...)
	return tag, false, err
}
