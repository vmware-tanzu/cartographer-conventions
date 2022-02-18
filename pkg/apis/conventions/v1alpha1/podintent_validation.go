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
	"fmt"

	"github.com/vmware-labs/reconciler-runtime/validation"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// +kubebuilder:webhook:path=/validate-conventions-carto-run-v1alpha1-podintent,mutating=false,failurePolicy=fail,sideEffects=none,admissionReviewVersions=v1beta1,groups=conventions.carto.run,resources=podintents,verbs=create;update,versions=v1alpha1,name=podintents.conventions.carto.run

var (
	_ webhook.Validator         = &PodIntent{}
	_ validation.FieldValidator = &PodIntent{}
)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *PodIntent) ValidateCreate() error {
	return r.Validate().ToAggregate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *PodIntent) ValidateUpdate(old runtime.Object) error {
	// TODO check for immutable fields
	return r.Validate().ToAggregate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *PodIntent) ValidateDelete() error {
	return nil
}

func (r *PodIntent) Validate() validation.FieldErrors {
	errs := validation.FieldErrors{}

	errs = errs.Also(r.Spec.Validate().ViaField("spec"))

	return errs
}

func (s *PodIntentSpec) Validate() validation.FieldErrors {
	errs := validation.FieldErrors{}
	for index, ips := range s.ImagePullSecrets {
		if ips.Name == "" {
			errs = errs.Also(validation.ErrMissingField(fmt.Sprintf("imagePullSecrets[%d].name", index)))
		}
	}
	// TODO

	return errs
}
