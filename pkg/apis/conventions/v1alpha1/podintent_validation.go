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
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-conventions-carto-run-v1alpha1-podintent,mutating=false,failurePolicy=fail,sideEffects=none,admissionReviewVersions=v1beta1,groups=conventions.carto.run,resources=podintents,verbs=create;update,versions=v1alpha1,name=podintents.conventions.carto.run

var (
	_ webhook.Validator = &PodIntent{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *PodIntent) ValidateCreate() (admission.Warnings, error) {
	return nil, r.validate().ToAggregate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *PodIntent) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	// TODO check for immutable fields
	return nil, r.validate().ToAggregate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *PodIntent) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *PodIntent) validate() field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, r.Spec.validate(field.NewPath("spec"))...)

	return errs
}

func (s *PodIntentSpec) validate(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	for index, ips := range s.ImagePullSecrets {
		if ips.Name == "" {
			errs = append(errs, field.Required(fldPath.Child("imagePullSecrets").Index(index).Child("name"), ""))
		}
	}
	// TODO

	return errs
}
