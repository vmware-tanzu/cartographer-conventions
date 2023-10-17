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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilpointer "k8s.io/utils/pointer"
)

const WrongPriority PriorityLevel = "wrong-level"
const InvalidSelectorTarget SelectorTargetSource = "invalidSelectorTarget"

func strPtr(s string) *string { return &s }

var (
	InvalidFailureType admissionregistrationv1.FailurePolicyType = "Invalid"
	DefaultFailureType                                           = admissionregistrationv1.Fail
	validClientConfig                                            = admissionregistrationv1.WebhookClientConfig{
		URL: strPtr("https://example.com"),
	}
	validaServiceRef = admissionregistrationv1.ServiceReference{
		Namespace: "ns",
		Name:      "n",
		Path:      strPtr("/"),
		Port:      utilpointer.Int32Ptr(443),
	}
)

func TestClusterPodConventionDefault(t *testing.T) {
	tests := []struct {
		name string
		in   *ClusterPodConvention
		want *ClusterPodConvention
	}{{
		name: "with service ref",
		in: &ClusterPodConvention{
			Spec: ClusterPodConventionSpec{
				SelectorTarget: "PodTemplateSpec",
				Priority:       EarlyPriority,
				Webhook: &ClusterPodConventionWebhook{
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "test-name",
							Namespace: "test-ns",
						},
					},
				},
			},
		},
		want: &ClusterPodConvention{
			Spec: ClusterPodConventionSpec{
				SelectorTarget: "PodTemplateSpec",
				Priority:       EarlyPriority,
				Webhook: &ClusterPodConventionWebhook{
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "test-name",
							Namespace: "test-ns",
							Port:      utilpointer.Int32Ptr(443),
						},
					},
				},
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

func TestClusterPodConventionValidate(t *testing.T) {
	for _, c := range []struct {
		name     string
		target   *ClusterPodConvention
		expected field.ErrorList
	}{{
		name: "empty webhook",
		target: &ClusterPodConvention{
			Spec: ClusterPodConventionSpec{
				SelectorTarget: "PodTemplateSpec",
				Priority:       "Normal",
			},
		},
		expected: field.ErrorList{
			field.Required(field.NewPath("spec", "webhook"), ""),
		},
	},
		{
			name: "neither URL nor service",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Early",
					Webhook:        &ClusterPodConventionWebhook{},
				},
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("spec", "webhook", "clientConfig", "[url, service]"), "expected exactly one, got neither"),
			},
		}, {
			name: "only URL",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Normal",
					Selectors: []metav1.LabelSelector{
						{
							MatchLabels: map[string]string{"foo": "bar"},
						},
					},
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: validClientConfig,
					},
				},
			},
			expected: field.ErrorList{},
		}, {
			name: "only service",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Normal",
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &validaServiceRef,
						},
					},
				},
			},
			expected: field.ErrorList{},
		}, {
			name: "both url and service",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Late",
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							URL:     strPtr("https://example.com"),
							Service: &validaServiceRef,
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("spec", "webhook", "clientConfig", "[url, service]"), "expected exactly one, got both"),
			},
		}, {
			name: "incomplete service",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Late",
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Port: validaServiceRef.Port,
							},
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("spec", "webhook", "clientConfig", "service", "name"), "service name is required"),
				field.Required(field.NewPath("spec", "webhook", "clientConfig", "service", "namespace"), "service namespace is required"),
			},
		}, {
			name: "invalid URL",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Early",
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							URL: strPtr("://example.com"),
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("spec", "webhook", "clientConfig", "url"), "url must be a valid URL: parse \"://example.com\": missing protocol scheme; desired format: https://host[/path]"),
			},
		}, {
			name: "bad matching service",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Normal",
					Selectors: []metav1.LabelSelector{
						{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Values:   []string{"foo", "bar"},
								Operator: metav1.LabelSelectorOpIn,
							}},
						},
					},
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: validClientConfig,
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "selectors").Index(0), metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Values:   []string{"foo", "bar"},
						Operator: metav1.LabelSelectorOpIn,
					}},
				}, ""),
			},
		}, {
			name: "with certificate",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Normal",
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &validaServiceRef,
						},
						Certificate: &ClusterPodConventionWebhookCertificate{
							Namespace: "default",
							Name:      "my-cert",
						},
					},
				},
			},
			expected: field.ErrorList{},
		}, {
			name: "invalid certificate",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Normal",
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &validaServiceRef,
						},
						Certificate: &ClusterPodConventionWebhookCertificate{
							Namespace: "",
							Name:      "",
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("spec", "webhook", "certificate", "namespace"), ""),
				field.Required(field.NewPath("spec", "webhook", "certificate", "name"), ""),
			},
		}, {
			name: "invalid selector target",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "invalidSelectorTarget",
					Priority:       "Early",
					Selectors: []metav1.LabelSelector{{
						MatchLabels: map[string]string{"foo": "bar"},
					}},
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: validClientConfig,
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "selectorTarget"), InvalidSelectorTarget, `The value provided for the selectorTarget field is invalid. Accepted selectorTarget values include \"PodIntent\" and \"PodTemplateSpec\". The default value is set to \"PodTemplateSpec\"`),
			},
		},
		{
			name: "wrong priority level",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "wrong-level",
					Selectors: []metav1.LabelSelector{{
						MatchLabels: map[string]string{"foo": "bar"},
					}},
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: validClientConfig,
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "priority"), WrongPriority, `The priority value provided is invalid. Accepted priority values include \"Early\" or \"Normal\" or \"Late\". The default value is set to \"Normal\"`),
			},
		}, {
			name: "valid priority level",
			target: &ClusterPodConvention{
				Spec: ClusterPodConventionSpec{
					SelectorTarget: "PodTemplateSpec",
					Priority:       "Early",
					Selectors: []metav1.LabelSelector{{
						MatchLabels: map[string]string{"foo": "bar"},
					}},
					Webhook: &ClusterPodConventionWebhook{
						ClientConfig: validClientConfig,
					},
				},
			},
			expected: field.ErrorList{},
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
				t.Errorf("ValidateDelete() (-expected, +actual) = %v", diff)
			}
		})
	}
}
