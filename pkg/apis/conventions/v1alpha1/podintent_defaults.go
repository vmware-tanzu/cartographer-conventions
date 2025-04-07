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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// +kubebuilder:webhook:path=/mutate-conventions-carto-run-v1alpha1-podintent,mutating=true,failurePolicy=fail,sideEffects=none,admissionReviewVersions=v1beta1,groups=conventions.carto.run,resources=podintents,verbs=create;update,versions=v1alpha1,name=podintents.conventions.carto.run

type PodIntentDefaulter struct{}

var _ webhook.CustomDefaulter = &PodIntentDefaulter{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *PodIntentDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	podIntent, ok := obj.(*PodIntent)
	if !ok {
		return fmt.Errorf("expected a PodIntent object, but got %T", obj)
	}
	return podIntent.Spec.Default()
}

func (s *PodIntentSpec) Default() error {
	if s.ServiceAccountName == "" {
		s.ServiceAccountName = "default"
	}
	return nil
}
