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

package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiserverwebhook "k8s.io/apiserver/pkg/util/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-conventions-carto-run-v1alpha1-clusterpodconvention,mutating=false,failurePolicy=fail,sideEffects=none,admissionReviewVersions=v1beta1,groups=conventions.carto.run,resources=clusterpodconventions,verbs=create;update,versions=v1alpha1,name=clusterpodconventions.conventions.carto.run

var (
	_ webhook.Validator = &ClusterPodConvention{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterPodConvention) ValidateCreate() (admission.Warnings, error) {
	return nil, r.validate().ToAggregate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *ClusterPodConvention) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	// TODO check for immutable fields
	return nil, c.validate().ToAggregate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *ClusterPodConvention) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *ClusterPodConvention) validate() field.ErrorList {
	errs := field.ErrorList{}
	errs = append(errs, r.Spec.validate(field.NewPath("spec"))...)
	return errs
}

func (s *ClusterPodConventionSpec) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	for i := range s.Selectors {
		if _, err := metav1.LabelSelectorAsSelector(&s.Selectors[i]); err != nil {
			errs = append(errs, field.Invalid(fldPath.Child("selectors").Index(i), s.Selectors[i], ""))
		}
	}

	if s.Priority != EarlyPriority && s.Priority != LatePriority && s.Priority != NormalPriority {
		errs = append(errs, field.Invalid(fldPath.Child("priority"), s.Priority, `The priority value provided is invalid. Accepted priority values include \"Early\" or \"Normal\" or \"Late\". The default value is set to \"Normal\"`))
	}

	// Webhook will be required mutually exclusive of other options that don't exist yet
	if s.Webhook == nil {
		errs = append(errs, field.Required(fldPath.Child("webhook"), ""))
	} else {
		errs = append(errs, s.Webhook.validate(fldPath.Child("webhook"))...)
	}

	if s.SelectorTarget != PodTemplateSpecLabels && s.SelectorTarget != PodIntentLabels {
		errs = append(errs,
			field.Invalid(fldPath.Child("selectorTarget"), s.SelectorTarget,
				`The value provided for the selectorTarget field is invalid. Accepted selectorTarget values include \"PodIntent\" and \"PodTemplateSpec\". The default value is set to \"PodTemplateSpec\"`),
		)
	}

	return errs
}

func (s *ClusterPodConventionWebhook) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, validateClientConfig(fldPath.Child("clientConfig"), s.ClientConfig)...)
	errs = append(errs, s.Certificate.validate(fldPath.Child("certificate"))...)

	return errs
}

func (s *ClusterPodConventionWebhookCertificate) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if s == nil {
		return errs
	}
	if s.Namespace == "" {
		errs = append(errs, field.Required(fldPath.Child("namespace"), ""))
	}
	if s.Name == "" {
		errs = append(errs, field.Required(fldPath.Child("name"), ""))
	}

	return errs
}

func validateClientConfig(fldPath *field.Path, clientConfig admissionregistrationv1.WebhookClientConfig) field.ErrorList {
	errs := field.ErrorList{}

	switch {
	case (clientConfig.URL != nil) && (clientConfig.Service != nil):
		errs = append(errs, field.Required(fldPath.Child("[url, service]"), "expected exactly one, got both"))
	case (clientConfig.URL == nil) == (clientConfig.Service == nil):
		errs = append(errs, field.Required(fldPath.Child("[url, service]"), "expected exactly one, got neither"))
	case clientConfig.URL != nil:
		errs = append(errs, apiserverwebhook.ValidateWebhookURL(fldPath.Child("url"), *clientConfig.URL, true)...)
	case clientConfig.Service != nil:
		errs = append(errs, apiserverwebhook.ValidateWebhookService(fldPath.Child("service"), clientConfig.Service.Name, clientConfig.Service.Namespace,
			clientConfig.Service.Path, *clientConfig.Service.Port)...)
	}

	return errs
}
