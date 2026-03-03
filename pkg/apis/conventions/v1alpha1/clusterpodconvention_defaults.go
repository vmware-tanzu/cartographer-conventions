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

	utilpointer "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-conventions-carto-run-v1alpha1-clusterpodconvention,mutating=true,failurePolicy=fail,sideEffects=none,admissionReviewVersions=v1beta1,groups=conventions.carto.run,resources=clusterpodconventions,verbs=create;update,versions=v1alpha1,name=clusterpodconventions.conventions.carto.run

type ClusterPodConventionDefaults struct{}

var _ admission.Defaulter[*ClusterPodConvention] = &ClusterPodConventionDefaults{}

func (*ClusterPodConventionDefaults) Default(ctx context.Context, obj *ClusterPodConvention) error {
	return obj.Spec.Default()
}

func (s *ClusterPodConventionSpec) Default() error {
	if s.Priority == "" {
		s.Priority = NormalPriority
	}
	if s.Webhook != nil {
		s.Webhook.Default()
	}
	if s.SelectorTarget == "" {
		s.SelectorTarget = PodTemplateSpecLabels
	}
	return nil
}

func (s *ClusterPodConventionWebhook) Default() {
	if s.ClientConfig.Service != nil {
		if s.ClientConfig.Service.Port == nil {
			s.ClientConfig.Service.Port = utilpointer.Int32Ptr(443)
		}
	}
}
