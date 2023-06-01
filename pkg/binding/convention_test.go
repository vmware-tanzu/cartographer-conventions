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

package binding_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	webhooktesting "k8s.io/apiserver/pkg/admission/plugin/webhook/testing"
	"k8s.io/apiserver/pkg/util/webhook"

	"github.com/vmware-tanzu/cartographer-conventions/pkg/binding"
	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

var (
	labelKey             = "test-convention"
	defaultLabel         = labelKey + "/default-label"
	deafultContainerName = "test-workload"
	defaultImageName     = "ubuntu"
	defaultEnvVar        = corev1.EnvVar{
		Name:  "KEY",
		Value: "VALUE",
	}
)

func strPtr(s string) *string { return &s }

func intPtr(s int32) *int32 { return &s }
func TestNewWebhookClientConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   *binding.Convention
		expects webhook.ClientConfig
	}{{
		name: "convention with name",
		input: &binding.Convention{
			Name: "test-os",
		},
		expects: webhook.ClientConfig{
			Name: "test-os",
		},
	}, {
		name: "convention with URL",
		input: &binding.Convention{
			Name: "test-os",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL: strPtr("http://localhost:80"),
			},
		},
		expects: webhook.ClientConfig{
			Name: "test-os",
			URL:  "http://localhost:80",
		},
	}, {
		name: "convention with service",
		input: &binding.Convention{
			Name: "test-os",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name:      "svc-name",
					Namespace: "svc-ns",
					Path:      strPtr("default"),
					Port:      intPtr(443),
				},
			},
		},
		expects: webhook.ClientConfig{
			Name: "test-os",
			Service: &webhook.ClientConfigService{
				Name:      "svc-name",
				Namespace: "svc-ns",
				Path:      "default",
				Port:      443,
			},
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := test.input.WebhookClientConfig()
			if diff := cmp.Diff(test.expects, actual); diff != "" {
				t.Errorf("NewWebhookClientConfig() (-expected, +actual) = %v", diff)
			}
		})
	}
}
func TestConvention(t *testing.T) {
	testServer := NewTestServer(t)
	testServer.Start()
	defer testServer.Close()
	serverURL, err := url.ParseRequestURI(testServer.URL)
	if err != nil {
		t.Fatalf("this should never happen? %v", err)
	}
	tests := []struct {
		name              string
		convention        binding.Convention
		conventionContext webhookv1alpha1.PodConventionContextSpec
		expects           webhookv1alpha1.PodConventionContextStatus
		expectsErr        bool
	}{{
		name: "convention with external URL",
		convention: binding.Convention{
			Name: "test-os",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL: strPtr(serverURL.String()),
			},
		},
		conventionContext: webhookv1alpha1.PodConventionContextSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "base", Image: "ubuntu",
					}},
				},
			},
		},
		expects: webhookv1alpha1.PodConventionContextStatus{
			AppliedConventions: []string{"test-convention/default-label"},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "base", Image: "ubuntu",
					}, {
						Name: deafultContainerName, Image: defaultImageName, Env: []corev1.EnvVar{defaultEnvVar},
					}},
				},
			},
		},
	}, {
		name: "convention with external URL that adds readyprobe",
		convention: binding.Convention{
			Name: "test-os",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL: strPtr(fmt.Sprintf("%s/%s", serverURL, "readyprobe")),
			},
		},
		conventionContext: webhookv1alpha1.PodConventionContextSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "base", Image: "ubuntu",
					}},
				},
			},
		},
		expects: webhookv1alpha1.PodConventionContextStatus{
			AppliedConventions: []string{"test-convention/default-label", "path/probe"},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "base", Image: "ubuntu",
					}, {
						Name:  "ready-probe-container",
						Image: defaultImageName,
						ReadinessProbe: &corev1.Probe{
							InitialDelaySeconds: 10,
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "",
									Port: intstr.FromInt(80),
								},
							},
						},
					}},
				},
			},
		},
	}, {
		name: "convention server with wrong content type",
		convention: binding.Convention{
			Name: "test-os",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL: strPtr(fmt.Sprintf("%s/%s", serverURL, "wrongcontenttype")),
			},
		},
		conventionContext: webhookv1alpha1.PodConventionContextSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "base", Image: "ubuntu",
					}},
				},
			},
		},
		expectsErr: true,
	}, {
		name: "convention server with no service ref or URL",
		convention: binding.Convention{
			Name: "test-os",
		},
		conventionContext: webhookv1alpha1.PodConventionContextSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "base", Image: "ubuntu",
					}},
				},
			},
		},
		expectsErr: true,
	}, {
		name: "convention server with wrong object type",
		convention: binding.Convention{
			Name: "test-os",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL: strPtr(fmt.Sprintf("%s/%s", serverURL, "wrongobj")),
			},
		},
		conventionContext: webhookv1alpha1.PodConventionContextSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "base", Image: "ubuntu",
					}},
				},
			},
		},
		expectsErr: true,
	}, {
		name: "convention server with non 200 status code",
		convention: binding.Convention{
			Name: "test-os",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				URL: strPtr(fmt.Sprintf("%s/%s", serverURL, "wrongstatuscode")),
			},
		},
		conventionContext: webhookv1alpha1.PodConventionContextSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "base", Image: "ubuntu",
					}},
				},
			},
		},
		expectsErr: true,
	}, {
		name: "convention with non resolvable service ref",
		convention: binding.Convention{
			Name: "test-os",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Name:      "webhook-test",
					Namespace: "failResolve",
				},
			},
		},
		conventionContext: webhookv1alpha1.PodConventionContextSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "base", Image: "ubuntu",
					}},
				},
			},
		},
		expectsErr: true,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			fakeWc := binding.WebhookConfig{
				AuthInfoResolver: webhooktesting.NewAuthenticationInfoResolver(new(int32)),
				ServiceResolver:  NewServiceResolver(*serverURL),
			}
			conventionContext := &webhookv1alpha1.PodConventionContext{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-op-req",
				},
				Spec: test.conventionContext,
			}
			actualConventionContext, err := test.convention.Apply(ctx, conventionContext, fakeWc)
			if test.expectsErr != (err != nil) {
				t.Errorf("Apply() expected error, got %v", err)
			}
			if actualConventionContext != nil {
				expectedConventionContext := &webhookv1alpha1.PodConventionContext{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-op-req",
					},
					Status: test.expects,
				}
				if diff := cmp.Diff(expectedConventionContext, actualConventionContext); diff != "" {
					t.Errorf("Apply() (-expected, +actual) = %v", diff)
				}
			}
		})
	}

}

type serviceResolver struct {
	base url.URL
}

// NewServiceResolver returns a static service resolve that return the given URL or
// an error for the failResolve namespace.
func NewServiceResolver(base url.URL) webhook.ServiceResolver {
	return &serviceResolver{base}
}

func (f serviceResolver) ResolveEndpoint(namespace, name string, port int32) (*url.URL, error) {
	if namespace == "failResolve" {
		return nil, fmt.Errorf("couldn't resolve service location")
	}
	u := f.base
	return &u, nil
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	reqObj := &webhookv1alpha1.PodConventionContext{}
	if r.Body != nil {
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		decoder := json.NewDecoder(bytes.NewBuffer(reqBody))
		if derr := decoder.Decode(reqObj); derr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	reqMetadata := reqObj.ObjectMeta.DeepCopy()
	validResponse := &webhookv1alpha1.PodConventionContext{
		ObjectMeta: *reqMetadata,
		Status: webhookv1alpha1.PodConventionContextStatus{
			Template: reqObj.Spec.Template,
		},
	}

	switch r.URL.Path {
	case "/wrongcontenttype":
		w.Header().Set("Content-Type", "application/unrecognized")
	case "/wrongobj":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode("invalid-resp-string")
	case "/wrongstatuscode":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
	case "/readyprobe":
		w.Header().Set("Content-Type", "application/json")
		validResponse.Status.Template.Spec.Containers = append(validResponse.Status.Template.Spec.Containers, corev1.Container{
			Name:  "ready-probe-container",
			Image: defaultImageName,
			ReadinessProbe: &corev1.Probe{
				InitialDelaySeconds: 10,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "",
						Port: intstr.FromInt(80),
					},
				},
			},
		})
		validResponse.Status.AppliedConventions = []string{defaultLabel, "path/probe"}
		json.NewEncoder(w).Encode(validResponse)
	case "/":
		w.Header().Set("Content-Type", "application/json")
		validResponse.Status.Template.Spec.Containers = append(validResponse.Status.Template.Spec.Containers, corev1.Container{
			Name:  deafultContainerName,
			Image: defaultImageName,
			Env:   []corev1.EnvVar{defaultEnvVar},
		})
		validResponse.Status.AppliedConventions = []string{defaultLabel}
		json.NewEncoder(w).Encode(validResponse)
	}
}

// NewTestServer returns a webhook test HTTPS server with fixed webhook test certs.
func NewTestServer(t testing.TB) *httptest.Server {
	testServer := httptest.NewUnstartedServer(http.HandlerFunc(webhookHandler))
	return testServer
}
