/*
Copyright 2022 VMware Inc.

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
	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
)

// +die:object=true
type _ = conventionsv1alpha1.PodIntent

// +die
type _ = conventionsv1alpha1.PodIntentSpec

func (d *PodIntentSpecDie) ImagePullSecretsDie(pullSecrets ...*diecorev1.LocalObjectReferenceDie) *PodIntentSpecDie {
	return d.DieStamp(func(r *conventionsv1alpha1.PodIntentSpec) {
		r.ImagePullSecrets = make([]corev1.LocalObjectReference, len(pullSecrets))
		for i := range pullSecrets {
			r.ImagePullSecrets[i] = pullSecrets[i].DieRelease()
		}
	})
}

func (d *PodIntentSpecDie) TemplateDie(fn func(d *diecorev1.PodTemplateSpecDie)) *PodIntentSpecDie {
	return d.DieStamp(func(r *conventionsv1alpha1.PodIntentSpec) {
		d := diecorev1.PodTemplateSpecBlank.
			DieImmutable(false).
			DieFeedPtr(r.Template.AsPodTemplateSpec())
		fn(d)
		r.Template = *conventionsv1alpha1.NewPodTemplateSpec(d.DieReleasePtr())
	})
}

// +die
type _ = conventionsv1alpha1.PodIntentStatus

func (d *PodIntentStatusDie) ConditionsDie(conditions ...*diemetav1.ConditionDie) *PodIntentStatusDie {
	return d.DieStamp(func(r *conventionsv1alpha1.PodIntentStatus) {
		r.Conditions = make([]metav1.Condition, len(conditions))
		for i := range conditions {
			r.Conditions[i] = conditions[i].DieRelease()
		}
	})
}

func (d *PodIntentStatusDie) ObservedGeneration(generation int64) *PodIntentStatusDie {
	return d.DieStamp(func(r *conventionsv1alpha1.PodIntentStatus) {
		r.ObservedGeneration = generation
	})
}

func (d *PodIntentStatusDie) TemplateDie(fn func(d *diecorev1.PodTemplateSpecDie)) *PodIntentStatusDie {
	return d.DieStamp(func(r *conventionsv1alpha1.PodIntentStatus) {
		d := diecorev1.PodTemplateSpecBlank.
			DieImmutable(false).
			DieFeedPtr(r.Template.AsPodTemplateSpec())
		fn(d)
		r.Template = conventionsv1alpha1.NewPodTemplateSpec(d.DieReleasePtr())
	})
}

var (
	PodIntentConditionReadyBlank              = diemetav1.ConditionBlank.Type(conventionsv1alpha1.PodIntentConditionReady)
	PodIntentConditionConventionsAppliedBlank = diemetav1.ConditionBlank.Type(conventionsv1alpha1.PodIntentConditionConventionsApplied)
)
