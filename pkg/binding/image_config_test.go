/*
Copyright 2021 VMware Inc.

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

package binding_test

import (
	"context"
	"encoding/pem"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	corev1 "k8s.io/api/core/v1"

	"github.com/vmware-tanzu/cartographer-conventions/pkg/binding"
	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

var (
	HelloConfigFile = ggcrv1.ConfigFile{
		Architecture: "arm64",
		Created:      ggcrv1.Time{Time: time.Date(2021, 8, 2, 15, 5, 4, 996677418, time.UTC)},
		History: []ggcrv1.History{
			{
				Created:    ggcrv1.Time{Time: time.Date(2021, 8, 2, 15, 5, 4, 996677418, time.UTC)},
				CreatedBy:  "LABEL hello=world",
				Comment:    "buildkit.dockerfile.v0",
				EmptyLayer: true,
			},
			{
				Created:   ggcrv1.Time{Time: time.Date(2021, 8, 2, 15, 5, 4, 996677418, time.UTC)},
				CreatedBy: "COPY boilerplate.go.txt /LICENSE # buildkit",
				Comment:   "buildkit.dockerfile.v0",
			},
		},
		OS: "linux",
		RootFS: ggcrv1.RootFS{
			Type: "layers",
			DiffIDs: []ggcrv1.Hash{
				{
					Algorithm: "sha256",
					Hex:       "5182dc320cc6fe03eb7799dc883aaf96f56ee769bfdb17be49a29f5219f20204",
				},
			},
		},
		Config: ggcrv1.Config{
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			},
			Labels: map[string]string{
				"hello": "world",
			},
			WorkingDir: "/",
		},
	}
	HelloDigest = "sha256:fede69b4ce95775cc92af3605555c2078b9b6d5eb3fb45d2d67fd6ac7a0209b7"
)

func TestCreateImageConfigs(t *testing.T) {
	testServer := httptest.NewServer(registry.New())
	defer testServer.Close()

	u, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("Error parsing %q: %v", testServer.URL, err)
	}

	helloImg, _ := crane.Load(path.Join("..", "..", "hack", "hello.tar.gz"))
	_ = crane.Push(helloImg, fmt.Sprintf("%s/hello", u.Host))
	sbomLayer, _ := tarball.LayerFromFile(path.Join("..", "..", "hack", "hello-sbom-layer.tar.gz"))
	sbomDiffId, _ := sbomLayer.DiffID()
	helloSbomImg, _ := mutate.AppendLayers(helloImg, sbomLayer)
	helloSbomImgConfig, _ := helloSbomImg.ConfigFile()
	helloSbomImgConfig.Config.Labels["io.buildpacks.app.sbom"] = sbomDiffId.String()
	helloSbomImg, _ = mutate.ConfigFile(helloSbomImg, helloSbomImgConfig)
	_ = crane.Push(helloSbomImg, fmt.Sprintf("%s/hello:sbom", u.Host))
	helloSbomImgDigest, _ := helloSbomImg.Digest()
	helloSboms := []webhookv1alpha1.BOM{
		// comparisons against the Raw field are suppressed
		{Name: "cnb-app:/layers/sbom/launch/paketo-buildpacks_bellsoft-liberica/helper/sbom.syft.json"},
		{Name: "cnb-app:/layers/sbom/launch/paketo-buildpacks_bellsoft-liberica/jre/sbom.syft.json"},
		{Name: "cnb-app:/layers/sbom/launch/paketo-buildpacks_ca-certificates/helper/sbom.syft.json"},
		{Name: "cnb-app:/layers/sbom/launch/paketo-buildpacks_executable-jar/sbom.cdx.json"},
		{Name: "cnb-app:/layers/sbom/launch/paketo-buildpacks_executable-jar/sbom.syft.json"},
		{Name: "cnb-app:/layers/sbom/launch/paketo-buildpacks_spring-boot/helper/sbom.syft.json"},
		{Name: "cnb-app:/layers/sbom/launch/paketo-buildpacks_spring-boot/spring-cloud-bindings/sbom.syft.json"},
	}

	ctx := context.Background()
	keychain, err := k8schain.NewNoClient(context.Background())
	if err != nil {
		t.Fatalf("Unable to create k8s auth chain %v", err)
	}
	dir, err := os.MkdirTemp(os.TempDir(), "ggcr-cache")
	if err != nil {
		t.Fatalf("Unable to create temp dir %v", err)
	}
	defer os.RemoveAll(dir)

	testCache := cache.NewFilesystemCache(dir)

	tests := []struct {
		name      string
		input     *corev1.PodTemplateSpec
		expects   []webhookv1alpha1.ImageConfig
		shouldErr bool
	}{{
		name:    "empty pod spec",
		input:   &corev1.PodTemplateSpec{},
		expects: nil,
	}, {
		name:    "nil pod spec",
		expects: nil,
	}, {
		name: "sbom",
		input: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "valid",
						Image: fmt.Sprintf("%s/hello:sbom", u.Host),
					},
				},
			},
		},
		expects: []webhookv1alpha1.ImageConfig{
			{
				Image:  fmt.Sprintf("%s/hello:sbom@%s", u.Host, helloSbomImgDigest.String()),
				Config: *helloSbomImgConfig,
				BOMs:   helloSboms,
			},
		},
	}, {
		name: "containers",
		input: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "valid",
						Image: fmt.Sprintf("%s/hello", u.Host),
					},
				},
			},
		},
		expects: []webhookv1alpha1.ImageConfig{
			{
				Image:  fmt.Sprintf("%s/hello:latest@%s", u.Host, HelloDigest),
				Config: HelloConfigFile,
			},
		},
	}, {
		name: "init containers",
		input: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:  "valid",
						Image: fmt.Sprintf("%s/hello", u.Host),
					},
				},
			},
		},
		expects: []webhookv1alpha1.ImageConfig{
			{
				Image:  fmt.Sprintf("%s/hello:latest@%s", u.Host, HelloDigest),
				Config: HelloConfigFile,
			},
		},
	}, {
		name: "digested image ref",
		input: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "valid",
						Image: fmt.Sprintf("%s/hello@%s", u.Host, HelloDigest),
					},
				},
			},
		},
		expects: []webhookv1alpha1.ImageConfig{
			{
				Image:  fmt.Sprintf("%s/hello@%s", u.Host, HelloDigest),
				Config: HelloConfigFile,
			},
		},
	}, {
		name: "mix of valid and invalid containers",
		input: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "valid",
						Image: fmt.Sprintf("%s/hello", u.Host),
					},
					{
						Name:  "invalid image",
						Image: fmt.Sprintf("%s/doesntexist", u.Host),
					},
				},
			},
		},
		shouldErr: true,
	}, {
		name: "mix of valid and invalid init containers",
		input: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:  "valid",
						Image: fmt.Sprintf("%s/hello", u.Host),
					},
					{
						Name:  "invalid image",
						Image: fmt.Sprintf("%s/doesntexist", u.Host),
					},
				},
			},
		},
		shouldErr: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rc := binding.RegistryConfig{
				Keys:  keychain,
				Cache: testCache,
			}

			actual, err := rc.ResolveImageMetadata(ctx, test.input)
			if test.shouldErr != (err != nil) {
				t.Errorf("ResolveImageMetadata() expected error: %v, but got: %v", test.shouldErr, err)
			}
			if test.shouldErr {
				return
			}
			ignoreRawBOM := cmpopts.IgnoreFields(webhookv1alpha1.BOM{}, "Raw")
			if diff := cmp.Diff(test.expects, actual, ignoreRawBOM); diff != "" {
				t.Errorf("ResolveImageMetadata() (-expected, +actual) = %v", diff)
			}
		})
	}
}

func TestImageConfigWithCustomCA(t *testing.T) {
	rs, err := registry.TLS("localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()

	rgUrl, err := url.Parse(rs.URL)
	if err != nil {
		t.Fatal(err)
	}

	image, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("Unable to make image: %v", err)
	}

	imageDigest, err := image.Digest()
	if err != nil {
		t.Fatalf("Unable to get image digest: %v", err)
	}

	digestedImage, err := name.NewDigest(rgUrl.Host + "/test@" + imageDigest.String())

	if err != nil {
		t.Fatalf("Unable to parse digest: %v", err)
	}
	if err := remote.Write(digestedImage, image, remote.WithTransport(rs.Client().Transport)); err != nil {
		t.Fatalf("Unable to push image to remote: %s", err)
	}

	// get the image ConfigFile for the cretaed image, used in test validation
	ref, err := name.ParseReference(digestedImage.Name())
	if err != nil {
		t.Fatalf("Unable to parse image name: %s", err)
	}
	img, err := remote.Image(ref, remote.WithTransport(rs.Client().Transport))
	if err != nil {
		t.Fatalf("Unable to get image: %s", err)
	}

	imageConfigFile, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("Unable to get image config file: %s", err)
	}

	cert, err := os.CreateTemp("", "cutomCA")
	if err != nil {
		t.Fatalf("Unable to create temp dir %v", err)
	}
	defer os.RemoveAll(cert.Name())

	if err := pem.Encode(cert, &pem.Block{Type: "CERTIFICATE", Bytes: rs.Certificate().Raw}); err != nil {
		t.Fatalf("Unable to parse certificate %v", err)
	}

	dir, err := os.MkdirTemp(os.TempDir(), "ggcr-cache")
	if err != nil {
		t.Fatalf("Unable to create temp dir %v", err)
	}
	defer os.RemoveAll(dir)

	ctx := context.Background()
	kc, err := k8schain.NewNoClient(ctx)
	if err != nil {
		t.Fatalf("Unable to create k8s auth chain %v", err)
	}
	testCache := cache.NewFilesystemCache(dir)

	tests := []struct {
		name      string
		input     *corev1.PodTemplateSpec
		rc        binding.RegistryConfig
		expects   []webhookv1alpha1.ImageConfig
		shouldErr bool
	}{{
		name: "private registry with cert",
		input: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-image",
						Image: digestedImage.Name(),
					},
				},
			},
		},
		rc: binding.RegistryConfig{Keys: kc, Cache: testCache, CACertPath: cert.Name()},
		expects: []webhookv1alpha1.ImageConfig{
			{
				Image:  digestedImage.String(),
				Config: *imageConfigFile,
			},
		},
		shouldErr: false,
	}, {
		name: "private registry without cert",
		input: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-image",
						Image: digestedImage.Name(),
					},
				},
			},
		},
		rc: binding.RegistryConfig{Keys: kc, Cache: testCache}, //no cert file
		expects: []webhookv1alpha1.ImageConfig{
			{
				Image:  digestedImage.String(),
				Config: *imageConfigFile,
			},
		},
		shouldErr: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := test.rc.ResolveImageMetadata(ctx, test.input)

			if test.shouldErr != (err != nil) {
				t.Errorf("ResolveImageMetadata() expected error: %v, but got: %v", test.shouldErr, err)
			}
			if test.shouldErr {
				return
			}
			if diff := cmp.Diff(test.expects, actual); diff != "" {
				t.Errorf("ResolveImageMetadata() (-expected, +actual) = %v", diff)
			}
		})
	}

}
