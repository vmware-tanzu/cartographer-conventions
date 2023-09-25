/*
Copyright 2020-2023 VMware Inc.

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

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/vmware-labs/reconciler-runtime/apis"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	"github.com/vmware-labs/reconciler-runtime/tracker"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
	certmanagerv1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/thirdparty/cert-manager/v1"
	"github.com/vmware-tanzu/cartographer-conventions/pkg/binding"
)

const (
	TLSCAKey = "ca.crt"
)

const (
	podIntentLabelsKey   = "PodIntent"
	podTemplateLabelsKey = "PodTemplateSpec"
)

var (
	secretGVK = schema.GroupVersionKind{
		Kind:    "Secret",
		Group:   corev1.SchemeGroupVersion.Group,
		Version: corev1.SchemeGroupVersion.Version,
	}
	certmanagerv1GVK = schema.GroupVersionKind{
		Kind:    "Certificate",
		Group:   certmanagerv1.SchemeGroupVersion.Group,
		Version: certmanagerv1.SchemeGroupVersion.Version,
	}
	serviceAccountGVK = schema.GroupVersionKind{
		Kind:    "ServiceAccount",
		Group:   corev1.SchemeGroupVersion.Group,
		Version: corev1.SchemeGroupVersion.Version,
	}
)

// +kubebuilder:rbac:groups=conventions.carto.run,resources=podintents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=conventions.carto.run,resources=podintents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

func PodIntentReconciler(c reconcilers.Config, wc binding.WebhookConfig, rc binding.RegistryConfig) *reconcilers.ResourceReconciler[*conventionsv1alpha1.PodIntent] {
	return &reconcilers.ResourceReconciler[*conventionsv1alpha1.PodIntent]{
		Name: "PodIntent",
		Reconciler: reconcilers.Sequence[*conventionsv1alpha1.PodIntent]{
			ResolveConventions(),
			BuildRegistryConfig(rc),
			ApplyConventionsReconciler(wc),
		},

		Config: c,
	}
}

// +kubebuilder:rbac:groups=conventions.carto.run,resources=clusterpodconventions,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificaterequests,verbs=get;list;watch

func ResolveConventions() reconcilers.SubReconciler[*conventionsv1alpha1.PodIntent] {
	return &reconcilers.SyncReconciler[*conventionsv1alpha1.PodIntent]{
		Name: "ResolveConventions",
		Sync: func(ctx context.Context, parent *conventionsv1alpha1.PodIntent) error {
			log := logr.FromContextOrDiscard(ctx)
			c := reconcilers.RetrieveConfigOrDie(ctx)
			sources := &conventionsv1alpha1.ClusterPodConventionList{}
			if err := c.List(ctx, sources); err != nil {
				return err
			}
			var conventions binding.Conventions
			conditionManager := parent.GetConditionSet().Manage(&parent.Status)
			for i := range sources.Items {
				source := sources.Items[i].DeepCopy()
				source.Default()
				convention := binding.Convention{
					Name:      source.Name,
					Selectors: source.Spec.Selectors,
					Priority:  source.Spec.Priority,
				}
				if source.Spec.Webhook != nil {
					clientConfig := source.Spec.Webhook.ClientConfig.DeepCopy()
					if source.Spec.Webhook.Certificate != nil {
						caBundle, err := getCABundle(ctx, c, source.Spec.Webhook.Certificate, parent, source)
						if err != nil {
							conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "CABundleResolutionFailed", "failed to authenticate: %v", err.Error())
							log.Error(err, "failed to get CABundle", "ClusterPodConvention", source.Name)
							return nil
						}
						// inject the CA data
						clientConfig.CABundle = caBundle
					}
					convention.ClientConfig = *clientConfig
				}
				conventions = append(conventions, convention)
			}
			StashConventions(ctx, conventions)
			return nil
		},

		Setup: func(ctx context.Context, mgr ctrl.Manager, bldr *builder.Builder) error {
			// register an informer to watch ClusterPodConventions
			bldr.Watches(&conventionsv1alpha1.ClusterPodConvention{}, &handler.Funcs{})
			bldr.Watches(&certmanagerv1.CertificateRequest{}, reconcilers.EnqueueTracked(ctx))

			return nil
		},
	}
}

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch

func BuildRegistryConfig(rc binding.RegistryConfig) reconcilers.SubReconciler[*conventionsv1alpha1.PodIntent] {
	return &reconcilers.SyncReconciler[*conventionsv1alpha1.PodIntent]{
		Name: "BuildRegistryConfig",
		SyncWithResult: func(ctx context.Context, parent *conventionsv1alpha1.PodIntent) (ctrl.Result, error) {
			log := logr.FromContextOrDiscard(ctx)
			c := reconcilers.RetrieveConfigOrDie(ctx)
			if rc.Client == nil {
				return ctrl.Result{}, fmt.Errorf("kubernetes client is not set")
			}
			conditionManager := parent.GetConditionSet().Manage(&parent.Status)

			var imagePullSecrets []string
			for _, ips := range parent.Spec.ImagePullSecrets {
				imagePullSecrets = append(imagePullSecrets, ips.Name)
				// track ref for updates
				ref := tracker.Reference{
					Kind:      secretGVK.Kind,
					APIGroup:  secretGVK.Group,
					Namespace: parent.Namespace,
					Name:      ips.Name,
				}
				c.Tracker.TrackReference(ref, parent)
			}

			serviceAccountName := parent.Spec.ServiceAccountName

			kc, err := k8schain.New(ctx, rc.Client, k8schain.Options{
				Namespace:          parent.Namespace,
				ServiceAccountName: serviceAccountName,
				ImagePullSecrets:   imagePullSecrets,
			})
			if err != nil {
				conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "ImageResolutionFailed", "failed to authenticate: %v", err.Error())
				log.Error(err, "fetching authentication for Images failed")
				return ctrl.Result{}, nil
			}

			serviceAccountNamespacedName := types.NamespacedName{Namespace: parent.Namespace, Name: serviceAccountName}
			sa := &corev1.ServiceAccount{}
			if err = c.TrackAndGet(ctx, serviceAccountNamespacedName, sa); err != nil {
				log.Error(err, "fetching serviceAccount failed")
				// should not happen mostly.
				conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "ImageResolutionFailed", fmt.Sprintf("failed to authenticate: %v", err.Error()))
				return ctrl.Result{}, nil
			}

			for _, secretReference := range sa.ImagePullSecrets {
				// track ref for updates
				ref := tracker.Reference{
					Kind:      secretGVK.Kind,
					APIGroup:  secretGVK.Group,
					Namespace: parent.Namespace,
					Name:      secretReference.Name,
				}
				c.Tracker.TrackReference(ref, parent)
			}

			StashRegistryConfig(ctx, binding.RegistryConfig{
				Keys:       kc,
				Cache:      rc.Cache,
				Client:     rc.Client,
				CACertPath: rc.CACertPath,
			})
			return ctrl.Result{}, nil
		},
		Setup: func(ctx context.Context, mgr reconcilers.Manager, bldr *reconcilers.Builder) error {
			// register an informer to watch Secret's metadata only. This reduces the cache size in memory.
			bldr.Watches(&corev1.Secret{}, reconcilers.EnqueueTracked(ctx), builder.OnlyMetadata)
			// register an informer to watch ServiceAccount
			bldr.Watches(&corev1.ServiceAccount{}, reconcilers.EnqueueTracked(ctx))
			return nil
		},
	}
}

func getCABundle(ctx context.Context, c reconcilers.Config, certRef *conventionsv1alpha1.ClusterPodConventionWebhookCertificate, parent *conventionsv1alpha1.PodIntent, convention *conventionsv1alpha1.ClusterPodConvention) ([]byte, error) {
	allCertReqs := &certmanagerv1.CertificateRequestList{}
	if err := c.List(ctx, allCertReqs, client.InNamespace(certRef.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to fetch associated `CertificateRequests` using the certificate namespace %q: %v configured on the ClusterPodConvention config", certRef.Namespace, err)
	}

	certReqs := []certmanagerv1.CertificateRequest{}
	for _, certReq := range allCertReqs.Items {
		if certReq.Annotations == nil || certReq.Annotations["cert-manager.io/certificate-name"] != certRef.Name {
			// request is for a different certificate
			continue
		}
		if len(certReq.Status.CA) == 0 {
			// request is missing a CA
			continue
		}
		readyFound := false
		for _, c := range certReq.Status.Conditions {
			if c.Type == certmanagerv1.CertificateRequestConditionReady && c.Status == metav1.ConditionTrue {
				readyFound = true
			}
		}
		if !readyFound {
			// request is not ready
			continue
		}
		certReqs = append(certReqs, certReq)
	}

	if len(certReqs) == 0 {
		return nil, fmt.Errorf(`unable to find valid "CertificateRequests" for certificate %q configured in convention %q`, fmt.Sprintf("%s/%s", certRef.Namespace, certRef.Name), convention.Name)
	}

	// take the most recent 3 certificate request CAs
	sort.Slice(certReqs, func(i, j int) bool {
		return certReqs[j].CreationTimestamp.Before(&certReqs[i].CreationTimestamp)
	})
	caData := bytes.NewBuffer([]byte{})
	for i, certReq := range certReqs {
		if i >= 3 {
			continue
		}
		caData.Write(certReq.Status.CA)
	}

	return caData.Bytes(), nil
}

func ApplyConventionsReconciler(wc binding.WebhookConfig) reconcilers.SubReconciler[*conventionsv1alpha1.PodIntent] {
	return &reconcilers.SyncReconciler[*conventionsv1alpha1.PodIntent]{
		Name: "ApplyConventions",
		SyncWithResult: func(ctx context.Context, parent *conventionsv1alpha1.PodIntent) (ctrl.Result, error) {
			log := logr.FromContextOrDiscard(ctx)

			sources := RetrieveConventions(ctx)
			workload := &parent.Spec.Template

			conditionManager := parent.GetConditionSet().Manage(&parent.Status)
			conventionsAppliedCond := conditionManager.GetCondition(conventionsv1alpha1.PodIntentConditionConventionsApplied)
			if conventionsAppliedCond == nil || apis.ConditionIsFalse(conventionsAppliedCond) {
				return ctrl.Result{}, nil
			}

			collectedLabels := make(map[string]labels.Set)
			collectedLabels[podIntentLabelsKey] = labels.Set(parent.ObjectMeta.GetLabels())
			collectedLabels[podTemplateLabelsKey] = labels.Set(workload.GetLabels())

			filteredAndSortedConventions, err := sources.FilterAndSort(collectedLabels)
			if err != nil {
				conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "LabelSelector", "filtering conventions failed: %v", err.Error())
				log.Error(err, "failed to filter conventions")
				return ctrl.Result{}, nil
			}
			if workload.Annotations == nil {
				workload.Annotations = map[string]string{}
			}
			updatedWorkload, err := filteredAndSortedConventions.Apply(ctx, parent, wc, RetrieveRegistryConfig(ctx))
			if err != nil {
				conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "ConventionsApplied", "%v", err.Error())
				return ctrl.Result{Requeue: true}, nil
			}
			parent.Status.Template = conventionsv1alpha1.NewPodTemplateSpec(updatedWorkload)
			conditionManager.MarkTrue(conventionsv1alpha1.PodIntentConditionConventionsApplied, "Applied", "")

			return ctrl.Result{}, nil
		},
	}
}

const (
	ConventionsStashKey reconcilers.StashKey = "conventions.carto.run/Conventions"
	RegistryConfigKey   reconcilers.StashKey = "conventions.carto.run/RegistryConfig"
)

func StashConventions(ctx context.Context, conventions []binding.Convention) {
	reconcilers.StashValue(ctx, ConventionsStashKey, conventions)
}

func RetrieveConventions(ctx context.Context) binding.Conventions {
	value := reconcilers.RetrieveValue(ctx, ConventionsStashKey)
	if conventions, ok := value.([]binding.Convention); ok {
		return conventions
	}
	return nil
}

func StashRegistryConfig(ctx context.Context, rc binding.RegistryConfig) {
	reconcilers.StashValue(ctx, RegistryConfigKey, rc)
}

func RetrieveRegistryConfig(ctx context.Context) binding.RegistryConfig {
	value := reconcilers.RetrieveValue(ctx, RegistryConfigKey)
	if workload, ok := value.(binding.RegistryConfig); ok {
		return workload
	}
	return binding.RegistryConfig{}
}

func splitNamespacedName(nameStr string) types.NamespacedName {
	splitPoint := strings.IndexRune(nameStr, types.Separator)
	if splitPoint == -1 {
		return types.NamespacedName{Name: nameStr}
	}
	return types.NamespacedName{Namespace: nameStr[:splitPoint], Name: nameStr[splitPoint+1:]}
}
