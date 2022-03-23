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

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

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
	"sigs.k8s.io/controller-runtime/pkg/source"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
	certmanagerv1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/thirdparty/cert-manager/v1"
	"github.com/vmware-tanzu/cartographer-conventions/pkg/binding"
)

const (
	TLSCAKey = "ca.crt"
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

func PodIntentReconciler(c reconcilers.Config, wc binding.WebhookConfig, rc binding.RegistryConfig) *reconcilers.ParentReconciler {
	c.Log = c.Log.WithName("PodIntent")

	return &reconcilers.ParentReconciler{
		Type: &conventionsv1alpha1.PodIntent{},
		Reconciler: reconcilers.Sequence{
			ResolveConventions(c),
			BuildRegistryConfig(c, rc),
			ApplyConventionsReconciler(c, wc),
		},

		Config: c,
	}
}

// +kubebuilder:rbac:groups=conventions.carto.run,resources=clusterpodconventions,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificaterequests,verbs=get;list;watch

func ResolveConventions(c reconcilers.Config) reconcilers.SubReconciler {
	c.Log = c.Log.WithName("ResolveConventions")

	return &reconcilers.SyncReconciler{
		Sync: func(ctx context.Context, parent *conventionsv1alpha1.PodIntent) error {
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
						caBundle, err := getCABundle(ctx, c, source.Spec.Webhook.Certificate, parent)
						if err != nil {
							conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "CABundleResolutionFailed", "failed to authenticate: %v", err.Error())
							c.Log.Error(err, "failed to get CABundle", "ClusterPodConvention", source.Name)
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
			bldr.Watches(&source.Kind{Type: &conventionsv1alpha1.ClusterPodConvention{}}, &handler.Funcs{})
			bldr.Watches(&source.Kind{Type: &certmanagerv1.CertificateRequest{}}, reconcilers.EnqueueTracked(&certmanagerv1.CertificateRequest{}, c.Tracker, c.Scheme()))

			return nil
		},
		Config: c,
	}
}

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch

func BuildRegistryConfig(c reconcilers.Config, rc binding.RegistryConfig) reconcilers.SubReconciler {
	c.Log = c.Log.WithName("BuildRegistryConfig")
	return &reconcilers.SyncReconciler{
		Sync: func(ctx context.Context, parent *conventionsv1alpha1.PodIntent) (ctrl.Result, error) {
			if rc.Client == nil {
				return ctrl.Result{}, fmt.Errorf("kubernetes client is not set")
			}
			conditionManager := parent.GetConditionSet().Manage(&parent.Status)
			parentNamespacedName := types.NamespacedName{Namespace: parent.Namespace, Name: parent.Name}

			var imagePullSecrets []string
			for _, ips := range parent.Spec.ImagePullSecrets {
				imagePullSecrets = append(imagePullSecrets, ips.Name)
				// track ref for updates
				key := tracker.NewKey(secretGVK, types.NamespacedName{Namespace: parent.Namespace, Name: ips.Name})
				c.Tracker.Track(key, parentNamespacedName)
			}

			serviceAccountName := parent.Spec.ServiceAccountName

			kc, err := k8schain.New(ctx, rc.Client, k8schain.Options{
				Namespace:          parent.Namespace,
				ServiceAccountName: serviceAccountName,
				ImagePullSecrets:   imagePullSecrets,
			})
			if err != nil {
				conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "ImageResolutionFailed", "failed to authenticate: %v", err.Error())
				c.Log.Error(err, "fetching authentication for Images failed")
				return ctrl.Result{}, nil
			}

			serviceAccountNamespacedName := types.NamespacedName{Namespace: parent.Namespace, Name: serviceAccountName}

			// track ref for updates
			key := tracker.NewKey(serviceAccountGVK, serviceAccountNamespacedName)
			c.Tracker.Track(key, parentNamespacedName)
			sa := &corev1.ServiceAccount{}

			if err = c.Get(ctx, serviceAccountNamespacedName, sa); err != nil {
				c.Log.Error(err, "fetching serviceAccount failed")
				// should not happen mostly.
				conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "ImageResolutionFailed", fmt.Sprintf("failed to authenticate: %v", err.Error()))
				return ctrl.Result{}, nil
			}

			for _, secretReference := range sa.ImagePullSecrets {
				// track ref for updates
				key := tracker.NewKey(secretGVK, types.NamespacedName{Namespace: parent.Namespace, Name: secretReference.Name})
				c.Tracker.Track(key, parentNamespacedName)
			}

			StashRegistryConfig(ctx, binding.RegistryConfig{
				Keys:       kc,
				Cache:      rc.Cache,
				Client:     rc.Client,
				CACertPath: rc.CACertPath,
			})
			return ctrl.Result{}, nil
		},
		Config: c,
		Setup: func(ctx context.Context, mgr reconcilers.Manager, bldr *reconcilers.Builder) error {
			// register an informer to watch Secret
			bldr.Watches(&source.Kind{Type: &corev1.Secret{}}, reconcilers.EnqueueTracked(&corev1.Secret{}, c.Tracker, c.Scheme()))
			// register an informer to watch ServiceAccount
			bldr.Watches(&source.Kind{Type: &corev1.ServiceAccount{}}, reconcilers.EnqueueTracked(&corev1.ServiceAccount{}, c.Tracker, c.Scheme()))
			return nil
		},
	}
}

func getCABundle(ctx context.Context, c reconcilers.Config, certRef *conventionsv1alpha1.ClusterPodConventionWebhookCertificate, parent *conventionsv1alpha1.PodIntent) ([]byte, error) {
	allCertReqs := &certmanagerv1.CertificateRequestList{}
	if err := c.List(ctx, allCertReqs, client.InNamespace(certRef.Namespace)); err != nil {
		return nil, fmt.Errorf("Failed to fetch associated certificate requests: %v", err)
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
		return nil, fmt.Errorf("unable to find valid certificaterequests for certificate %q", fmt.Sprintf("%s/%s", certRef.Namespace, certRef.Name))
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

func ApplyConventionsReconciler(c reconcilers.Config, wc binding.WebhookConfig) reconcilers.SubReconciler {
	c.Log = c.Log.WithName("ApplyConventions")

	return &reconcilers.SyncReconciler{
		Sync: func(ctx context.Context, parent *conventionsv1alpha1.PodIntent) (ctrl.Result, error) {
			sources := RetrieveConventions(ctx)
			workload := &parent.Spec.Template

			conditionManager := parent.GetConditionSet().Manage(&parent.Status)
			conventionsAppliedCond := conditionManager.GetCondition(conventionsv1alpha1.PodIntentConditionConventionsApplied)
			if conventionsAppliedCond == nil || apis.ConditionIsFalse(conventionsAppliedCond) {
				return ctrl.Result{}, nil
			}

			filteredAndSortedConventions, err := sources.FilterAndSort(labels.Set(workload.GetLabels()))
			if err != nil {
				conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "LabelSelector", "filtering conventions failed: %v", err.Error())
				c.Log.Error(err, "failed to filter sources")
				return ctrl.Result{}, nil
			}
			if workload.Annotations == nil {
				workload.Annotations = map[string]string{}
			}
			updatedWorkload, err := filteredAndSortedConventions.Apply(ctx, c.Log, parent, wc, RetrieveRegistryConfig(ctx))
			if err != nil {
				conditionManager.MarkFalse(conventionsv1alpha1.PodIntentConditionConventionsApplied, "ConventionsApplied", "%v", err.Error())
				return ctrl.Result{Requeue: true}, nil
			}
			parent.Status.Template = conventionsv1alpha1.NewPodTemplateSpec(updatedWorkload)
			conditionManager.MarkTrue(conventionsv1alpha1.PodIntentConditionConventionsApplied, "Applied", "")

			return ctrl.Result{}, nil
		},

		Config: c,
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
