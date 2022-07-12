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

package v1alpha1

import (
	"github.com/vmware-labs/reconciler-runtime/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiserverwebhook "k8s.io/apiserver/pkg/util/webhook"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// +kubebuilder:webhook:path=/validate-conventions-carto-run-v1alpha1-clusterpodconvention,mutating=false,failurePolicy=fail,sideEffects=none,admissionReviewVersions=v1beta1,groups=conventions.carto.run,resources=clusterpodconventions,verbs=create;update,versions=v1alpha1,name=clusterpodconventions.conventions.carto.run

var (
	_ webhook.Validator         = &ClusterPodConvention{}
	_ validation.FieldValidator = &ClusterPodConvention{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterPodConvention) ValidateCreate() error {
	return r.Validate().ToAggregate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *ClusterPodConvention) ValidateUpdate(old runtime.Object) error {
	// TODO check for immutable fields
	return c.Validate().ToAggregate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *ClusterPodConvention) ValidateDelete() error {
	return nil
}

func (r *ClusterPodConvention) Validate() validation.FieldErrors {
	errs := validation.FieldErrors{}
	return errs.Also(r.Spec.Validate().ViaField("spec"))
}

func (s *ClusterPodConventionSpec) Validate() validation.FieldErrors {
	errs := validation.FieldErrors{}

	for i := range s.Selectors {
		if _, err := metav1.LabelSelectorAsSelector(&s.Selectors[i]); err != nil {
			errs = errs.Also(
				validation.ErrInvalidArrayValue(s.Selectors[i], "selector", i),
			)
		}
	}

	if s.Priority != EarlyPriority && s.Priority != LatePriority && s.Priority != NormalPriority {
		errs = errs.Also(validation.FieldErrors{
			field.Invalid(field.NewPath("priority"), s.Priority, "Accepted priority values \"Early\" or \"Normal\" or \"Late\""),
		})
	}

	// Webhook will be required mutually exclusive of other options that don't exist yet
	if s.Webhook == nil {
		errs = errs.Also(validation.ErrMissingField("webhook"))
	} else {
		errs = errs.Also(s.Webhook.Validate().ViaField("webhook"))
	}

	return errs
}

func (s *ClusterPodConventionWebhook) Validate() validation.FieldErrors {
	errs := validation.FieldErrors{}

	errs = errs.Also(ValidateClientConfig(s.ClientConfig).ViaField("clientConfig"))
	errs = errs.Also(s.Certificate.Validate().ViaField("certificate"))

	return errs
}

func (s *ClusterPodConventionWebhookCertificate) Validate() validation.FieldErrors {
	errs := validation.FieldErrors{}

	if s == nil {
		return errs
	}
	if s.Namespace == "" {
		errs = errs.Also(validation.ErrMissingField("namespace"))
	}
	if s.Name == "" {
		errs = errs.Also(validation.ErrMissingField("name"))
	}

	return errs
}

func ValidateClientConfig(clientConfig admissionregistrationv1.WebhookClientConfig) validation.FieldErrors {
	errs := validation.FieldErrors{}
	switch {
	case (clientConfig.URL != nil) && (clientConfig.Service != nil):
		errs = errs.Also(validation.ErrMultipleOneOf("url", "service"))
	case (clientConfig.URL == nil) == (clientConfig.Service == nil):
		errs = errs.Also(validation.ErrMissingOneOf("url", "service"))
	case clientConfig.URL != nil:
		errs = append(errs, apiserverwebhook.ValidateWebhookURL(field.NewPath("url"), *clientConfig.URL, true)...)
	case clientConfig.Service != nil:
		errs = append(errs, apiserverwebhook.ValidateWebhookService(field.NewPath("service"), clientConfig.Service.Name, clientConfig.Service.Namespace,
			clientConfig.Service.Path, *clientConfig.Service.Port)...)
	}
	return errs
}
