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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vmware-labs/reconciler-runtime/apis"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestPodIntentDefault(t *testing.T) {
	tests := []struct {
		name string
		in   *PodIntent
		want *PodIntent
	}{{
		name: "empty",
		in:   &PodIntent{},
		want: &PodIntent{
			Spec: PodIntentSpec{
				ServiceAccountName: "default",
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in
			got.Default()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Default() (-want, +got) = %v", diff)
			}
		})
	}
}

func TestPodIntentValidate(t *testing.T) {
	for _, c := range []struct {
		name     string
		target   *PodIntent
		expected field.ErrorList
	}{{
		name:     "empty",
		target:   &PodIntent{},
		expected: field.ErrorList{},
	}, {
		name: "empty image pull secret",
		target: &PodIntent{
			Spec: PodIntentSpec{
				ImagePullSecrets: []corev1.LocalObjectReference{{}},
			},
		},
		expected: field.ErrorList{
			field.Required(field.NewPath("spec", "imagePullSecrets").Index(0).Child("name"), ""),
		},
	}} {
		t.Run(c.name, func(t *testing.T) {
			actual := c.target.validate()
			if diff := cmp.Diff(c.expected, actual); diff != "" {
				t.Errorf("Validate() (-expected, +actual) = %v", diff)
			}
			_, create := c.target.ValidateCreate()
			if diff := cmp.Diff(c.expected.ToAggregate(), create); diff != "" {
				t.Errorf("ValidateCreate() (-expected, +actual) = %v", diff)
			}
			_, update := c.target.ValidateUpdate(nil)
			if diff := cmp.Diff(c.expected.ToAggregate(), update); diff != "" {
				t.Errorf("ValidateUpdate() (-expected, +actual) = %v", diff)
			}
			_, delete := c.target.ValidateDelete()
			if diff := cmp.Diff(nil, delete); diff != "" {
				t.Errorf("ValidateUpdate() (-expected, +actual) = %v", diff)
			}
		})
	}
}

func TestPodIntentConditions(t *testing.T) {
	for _, c := range []struct {
		name     string
		work     func(*PodIntent)
		expected *PodIntentStatus
	}{{
		name: "initialize",
		work: func(s *PodIntent) {
			s.Status.InitializeConditions()
		},
		expected: &PodIntentStatus{
			Status: apis.Status{
				Conditions: []metav1.Condition{
					{
						Type:   PodIntentConditionConventionsApplied,
						Status: metav1.ConditionUnknown,
						Reason: "Initializing",
					},
					{
						Type:   PodIntentConditionReady,
						Status: metav1.ConditionUnknown,
						Reason: "Initializing",
					},
				},
			},
		},
	}, {
		name: "reset",
		work: func(s *PodIntent) {
			s.GetConditionSet().Manage(s.GetConditionsAccessor()).MarkTrue(PodIntentConditionConventionsApplied, "Applied", "")
			s.Status.InitializeConditions()
		},
		expected: &PodIntentStatus{
			Status: apis.Status{
				Conditions: []metav1.Condition{
					{
						Type:   PodIntentConditionConventionsApplied,
						Status: metav1.ConditionUnknown,
						Reason: "Initializing",
					},
					{
						Type:   PodIntentConditionReady,
						Status: metav1.ConditionUnknown,
						Reason: "Initializing",
					},
				},
			},
		},
	}, {
		name: "ready",
		work: func(s *PodIntent) {
			s.Status.InitializeConditions()
			s.GetConditionSet().Manage(s.GetConditionsAccessor()).MarkTrue(PodIntentConditionConventionsApplied, "Applied", "")
		},
		expected: &PodIntentStatus{
			Status: apis.Status{
				Conditions: []metav1.Condition{
					{
						Type:   PodIntentConditionConventionsApplied,
						Status: metav1.ConditionTrue,
						Reason: "Applied",
					},
					{
						Type:   PodIntentConditionReady,
						Status: metav1.ConditionTrue,
						Reason: "ConventionsApplied",
					},
				},
			},
		},
	}} {
		t.Run(c.name, func(t *testing.T) {
			actual := &PodIntent{}
			c.work(actual)
			if diff := cmp.Diff(c.expected, &actual.Status, rtesting.IgnoreLastTransitionTime); diff != "" {
				t.Errorf("(-expected, +actual) = %v", diff)
			}
		})
	}
}
