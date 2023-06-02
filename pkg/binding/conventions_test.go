/*
Copyright 2021-2023 VMware Inc.

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
	"context"
	"encoding/pem"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	webhooktesting "k8s.io/apiserver/pkg/admission/plugin/webhook/testing"
	"k8s.io/utils/pointer"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
	"github.com/vmware-tanzu/cartographer-conventions/pkg/binding"
	"github.com/vmware-tanzu/cartographer-conventions/pkg/binding/fake"
)

var (
	BadCACert = []byte(`-----BEGIN CERTIFICATE-----
MIIDPTCCAiWgAwIBAgIJAIaoBDrksTyaMA0GCSqGSIb3DQEBCwUAMDQxMjAwBgNV
BAMMKWdlbmVyaWNfd2ViaG9va19hZG1pc3Npb25fcGx1Z2luX3Rlc3RzX2NhMCAX
DTE3MTExNjAwMDUzOVoYDzIyOTEwOTAxMDAwNTM5WjA0MTIwMAYDVQQDDClnZW5l
cmljX3dlYmhvb2tfYWRtaXNzaW9uX3BsdWdpbl90ZXN0c19jYTCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBAK4GKGQFA49f3UNhvnTIm/m9Zt/QAiusAbeM
w45fMeYlGWYw8jtZfx4+p9zVB6YRbGGedO9HbPBsFwDb2BhYtxehhYkVv0eZXAoZ
ocYWOSSbVqrg6WpqJzRI4gLohX+rugingb5vAoHB/wm83OFz9aCWYkmhjqZqhoh5
S3i9ucumUd1+w4zj2pUovVh0DdJvQ0uxDL8mpckgMMySpXDqUngT3TE6dQMtR0oS
YojY/LHQkS6au68B8qSkuplTSLbAJ8fo3ONHdetnZhUIPBQZtzOneUE6yQQH7r6C
65TQrbLJddYTolw2CbIUwVSPRwEf5c0IhfnKGThycGLmF6e8WDsCAwEAAaNQME4w
HQYDVR0OBBYEFFFthspVCOb5fSkQ2BFCykech3RVMB8GA1UdIwQYMBaAFFFthspV
COb5fSkQ2BFCykech3RVMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEB
AEgGbcx1qhdi4lFNC0YRHJxjn3JPW6tr4qgDiusqMj9TF9/RohKOvLblq2kSB0x3
pyDMkVv2rd5U4qtKruEQ1OgY3cB7hy6mt/ZhldF540Lli8j9N63LMRXwIu068j2W
WSiWV416LOZEcuid7mZjAsbG4xvaDg/yW1RBpA3XnwMSmr7Y+T6XkjzgT3WWiwOf
4ANc3ecsl53x/beb9YF+TjqmjmtGSgUW78UTAsGFFKmjJ/cStQUaMCEvS9Gun7hH
eLarZIVV5Ia/FziGHoi7Q44C66pXD437xmkR1ueExoKwXbBt4c5GeH1rJjUVnlyk
pMokZBC57nXx8krZVEu1SRA=
-----END CERTIFICATE-----`)
)

func TestConventionFilter(t *testing.T) {
	tests := []struct {
		name            string
		input           []binding.Convention
		collectedLabels map[string]labels.Set
		expects         []binding.Convention
		expectErr       bool
	}{{
		name: "select all conventions",
		collectedLabels: map[string]labels.Set{
			"PodTemplateSpec": map[string]string{"foo": "bar"},
			"PodIntent":       map[string]string{},
		},
		input: []binding.Convention{{
			Name:           "test",
			SelectorTarget: "PodTemplateSpec",
			Selectors: []metav1.LabelSelector{{
				MatchLabels: map[string]string{"foo": "bar"},
			}},
		}},
		expects: []binding.Convention{{
			Name:           "test",
			SelectorTarget: "PodTemplateSpec",
			Selectors: []metav1.LabelSelector{{
				MatchLabels: map[string]string{"foo": "bar"},
			}},
		}},
	}, {
		name: "source with no labels",
		collectedLabels: map[string]labels.Set{
			"PodTemplateSpec": map[string]string{"foo": "bar"},
			"PodIntent":       map[string]string{},
		},
		input: []binding.Convention{{
			Name: "test",
		}},
		expects: []binding.Convention{{
			Name: "test",
		}},
	}, {
		name: "source with mix of labels and no labels",
		collectedLabels: map[string]labels.Set{
			"PodTemplateSpec": map[string]string{"foo": "bar"},
			"PodIntent":       map[string]string{},
		},
		input: []binding.Convention{{
			Name: "test",
		}, {
			Name:           "test1",
			SelectorTarget: "PodTemplateSpec",
			Selectors: []metav1.LabelSelector{{
				MatchLabels: map[string]string{"foo": "bar"},
			}},
		}},
		expects: []binding.Convention{{
			Name: "test",
		}, {
			Name:           "test1",
			SelectorTarget: "PodTemplateSpec",
			Selectors: []metav1.LabelSelector{{
				MatchLabels: map[string]string{"foo": "bar"},
			}},
		}},
	},
		{
			name: "workload with no labels",
			collectedLabels: map[string]labels.Set{
				"PodTemplateSpec": map[string]string{},
				"PodIntent":       map[string]string{},
			},
			input: []binding.Convention{{
				Name:           "test",
				SelectorTarget: "PodTemplateSpec",
				Selectors: []metav1.LabelSelector{{
					MatchLabels: map[string]string{"foo": "bar"},
				}},
			}},
		}, {
			name: "source with invalid labels",
			collectedLabels: map[string]labels.Set{
				"PodTemplateSpec": map[string]string{"foo": "bar"},
				"PodIntent":       map[string]string{},
			},
			input: []binding.Convention{{
				Name:           "test",
				SelectorTarget: "PodTemplateSpec",
				Selectors: []metav1.LabelSelector{{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      "baz",
						Operator: metav1.LabelSelectorOpExists,
						Values:   []string{"qux", "norf"},
					}},
				}},
			}},
			expectErr: true,
		}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actual, expects binding.Conventions
			actual = test.input
			expects = test.expects
			filteredConventions, err := actual.FilterAndSort(test.collectedLabels)
			if err == nil && test.expectErr {
				t.Error("expected error but got none.")
			}
			if err != nil && !test.expectErr {
				t.Errorf("did not expect error but got: %v", err)
			}
			if diff := cmp.Diff(expects, filteredConventions); !test.expectErr && diff != "" {
				t.Errorf("Filter() (-expected, +actual) = %v", diff)
			}
		})
	}
}

func TestConventionOrder(t *testing.T) {
	tests := []struct {
		name    string
		input   []binding.Convention
		expects []binding.Convention
	}{{
		name: "same name, different priority",
		input: []binding.Convention{{
			Name:     "xyz",
			Priority: conventionsv1alpha1.NormalPriority,
		}, {
			Name:     "xyz",
			Priority: conventionsv1alpha1.EarlyPriority,
		}},
		expects: []binding.Convention{{
			Name:     "xyz",
			Priority: conventionsv1alpha1.EarlyPriority,
		}, {
			Name:     "xyz",
			Priority: conventionsv1alpha1.NormalPriority,
		}},
	}, {
		name: "combination of priority",
		input: []binding.Convention{{
			Name:     "test-low-1",
			Priority: conventionsv1alpha1.LatePriority,
		}, {
			Name:     "test2",
			Priority: conventionsv1alpha1.EarlyPriority,
		}, {
			Name:     "test-low-2",
			Priority: conventionsv1alpha1.LatePriority,
		}, {
			Name:     "test-normal-priority",
			Priority: conventionsv1alpha1.NormalPriority,
		}},
		expects: []binding.Convention{{
			Name:     "test2",
			Priority: conventionsv1alpha1.EarlyPriority,
		}, {
			Name:     "test-normal-priority",
			Priority: conventionsv1alpha1.NormalPriority,
		}, {
			Name:     "test-low-1",
			Priority: conventionsv1alpha1.LatePriority,
		}, {
			Name:     "test-low-2",
			Priority: conventionsv1alpha1.LatePriority,
		}},
	}, {
		name: "same priority, different name",
		input: []binding.Convention{{
			Name:     "xyz",
			Priority: conventionsv1alpha1.NormalPriority,
		}, {
			Name:     "abc",
			Priority: conventionsv1alpha1.NormalPriority,
		}},
		expects: []binding.Convention{{
			Name:     "abc",
			Priority: conventionsv1alpha1.NormalPriority,
		}, {
			Name:     "xyz",
			Priority: conventionsv1alpha1.NormalPriority,
		}},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actual, expects binding.Conventions
			actual = test.input
			sortedConventions := actual.Sort()
			expects = test.expects

			if diff := cmp.Diff(expects, sortedConventions); diff != "" {
				t.Errorf("Order() (-expected, +actual) = %v", diff)
			}
			if !cmp.Equal(expects, sortedConventions) {
				t.Errorf("Order() (-expected: %v, +actual: %v)", expects, sortedConventions)
			}
		})
	}
}

func TestConventionApply(t *testing.T) {
	testServer, caCert, err := fake.NewFakeConventionServer()
	if err != nil {
		t.Fatalf("unable to create convention server: %v", err)
	}
	testServer.StartTLS()
	defer testServer.Close()

	serverURL, err := url.ParseRequestURI(testServer.URL)
	if err != nil {
		t.Fatalf("this should never happen? %v", err)
	}
	wc := binding.WebhookConfig{
		AuthInfoResolver: webhooktesting.NewAuthenticationInfoResolver(new(int32)),
		ServiceResolver:  fake.NewStubServiceResolver(*serverURL),
	}

	dir, err := os.MkdirTemp(os.TempDir(), "ggcr-cache")
	if err != nil {
		t.Fatalf("Unable to create temp dir %v", err)
	}
	defer os.RemoveAll(dir)

	testCache := cache.NewFilesystemCache(dir)
	kc, err := k8schain.NewNoClient(context.Background())
	if err != nil {
		t.Fatalf("Unable to create k8s auth chain %v", err)
	}
	registryServer := httptest.NewServer(registry.New())
	defer registryServer.Close()
	registryUrl, _ := url.Parse(registryServer.URL)

	img, err := crane.Load(path.Join("..", "..", "hack", "hello.tar.gz"))
	if err != nil {
		t.Fatalf("Error loading hello.tar.gz: %v", err)
	}
	if err := crane.Push(img, fmt.Sprintf("%s/hello", registryUrl.Host)); err != nil {
		t.Fatalf("Error pushing hello.tar.gz: %v", err)
	}

	rc := binding.RegistryConfig{Keys: kc, Cache: testCache}
	namespace := "test-namespace"
	name := "my-template"
	tests := []struct {
		name       string
		convetions []binding.Convention
		workload   *conventionsv1alpha1.PodIntent
		expects    *corev1.PodTemplateSpec
		shouldErr  bool
	}{{
		name: "valid case",
		convetions: []binding.Convention{{
			Name: "my-conventions",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Namespace: "default",
					Name:      "webhook-test",
				},
				CABundle: caCert,
			},
		}},
		workload: &conventionsv1alpha1.PodIntent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: conventionsv1alpha1.PodIntentSpec{},
		},
		expects: &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"conventions.carto.run/applied-conventions": "my-conventions/test-convention/default-label"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "test-workload",
					Image: "ubuntu",
					Env: []corev1.EnvVar{
						{
							Name:  "KEY",
							Value: "VALUE",
						},
					},
				}},
			},
		},
	}, {
		name: "workload with existing annotation case",
		convetions: []binding.Convention{{
			Name: "my-conventions",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Namespace: "default",
					Name:      "webhook-test",
				},
				CABundle: caCert,
			},
		}},
		workload: &conventionsv1alpha1.PodIntent{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   namespace,
				Annotations: map[string]string{"conventions.carto.run/applied-conventions": "existing-conventions/another-convention/default-convention"},
			},
			Spec: conventionsv1alpha1.PodIntentSpec{
				Template: conventionsv1alpha1.PodTemplateSpec{
					ObjectMeta: conventionsv1alpha1.ObjectMeta{
						Annotations: map[string]string{"conventions.carto.run/applied-conventions": "existing-conventions/another-convention/default-convention"},
					},
				},
			},
		},
		expects: &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"conventions.carto.run/applied-conventions": "existing-conventions/another-convention/default-convention\nmy-conventions/test-convention/default-label"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "test-workload",
					Image: "ubuntu",
					Env: []corev1.EnvVar{
						{
							Name:  "KEY",
							Value: "VALUE",
						},
					},
				}},
			},
		},
	}, {
		name: "only label",
		convetions: []binding.Convention{{
			Name: "my-conventions",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Namespace: "default",
					Name:      "webhook-test",
					Path:      pointer.String("labelonly"),
				},
				CABundle: caCert,
			},
		}},
		workload: &conventionsv1alpha1.PodIntent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: conventionsv1alpha1.PodIntentSpec{},
		},
		expects: &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"conventions.carto.run/applied-conventions": "my-conventions/test-convention/default-label\nmy-conventions/path/addonlylabel"},
			},
		},
	}, {
		name: "bad image",
		convetions: []binding.Convention{
			{
				Name:     "my-conventions",
				Priority: conventionsv1alpha1.NormalPriority,
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      &testServer.URL,
					CABundle: caCert,
				},
			},
		},
		workload: &conventionsv1alpha1.PodIntent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: conventionsv1alpha1.PodIntentSpec{
				Template: conventionsv1alpha1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Image: fmt.Sprintf("%s/badimage", registryUrl.Host),
						}},
					},
				},
			},
		},
		shouldErr: true,
	}, {
		name: "bad ca cert",
		convetions: []binding.Convention{
			{
				Name:     "my-conventions",
				Priority: conventionsv1alpha1.LatePriority,
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					URL:      &testServer.URL,
					CABundle: BadCACert,
				},
			},
		},
		workload: &conventionsv1alpha1.PodIntent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: conventionsv1alpha1.PodIntentSpec{},
		},
		shouldErr: true,
	}, {
		name: "bad image in-between resolving",
		convetions: []binding.Convention{{
			Name:     "my-conventions",
			Priority: conventionsv1alpha1.NormalPriority,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Namespace: "default",
					Name:      "webhook-test",
					Path:      pointer.String(fmt.Sprintf("badimage;host=%s", registryUrl.Host)),
				},
				CABundle: caCert,
			},
		},
			{
				Name: "label-conventions",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-test",
						Path:      pointer.String("labelonly"),
					},
					CABundle: caCert,
				},
			},
		},
		workload: &conventionsv1alpha1.PodIntent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: conventionsv1alpha1.PodIntentSpec{},
		},
		shouldErr: true,
	}, {
		name: "resolve image between conventions",
		convetions: []binding.Convention{
			{
				Name:     "additional-conventions",
				Priority: conventionsv1alpha1.NormalPriority,
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-test",
						Path:      pointer.String(fmt.Sprintf("hellosidecar;host=%s", registryUrl.Host)),
					},
					CABundle: caCert,
				},
			},
			{
				Name:     "my-conventions",
				Priority: conventionsv1alpha1.NormalPriority,
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "default",
						Name:      "webhook-test",
					},
					CABundle: caCert,
				},
			},
		},
		workload: &conventionsv1alpha1.PodIntent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: conventionsv1alpha1.PodIntentSpec{},
		},
		expects: &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"conventions.carto.run/applied-conventions": "additional-conventions/path/hellosidecar\nmy-conventions/test-convention/default-label"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "hellosidecar",
						Command: []string{"/bin/sleep", "100"},
						Image:   fmt.Sprintf("%s/hello:latest@%s", registryUrl.Host, HelloDigest),
					},
					{
						Name:  "test-workload",
						Image: "ubuntu",
						Env: []corev1.EnvVar{
							{
								Name:  "KEY",
								Value: "VALUE",
							},
						},
					},
				},
			},
		},
	}, {
		name:      "nil workload",
		shouldErr: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var input binding.Conventions
			input = append(input, test.convetions...)
			updatedSpec, err := input.Apply(context.Background(), test.workload, wc, rc)
			if (err != nil) != test.shouldErr {
				t.Errorf("Apply() error = %v, ExpectErr %v", err, test.shouldErr)
			}
			if diff := cmp.Diff(test.expects, updatedSpec); diff != "" {
				t.Errorf("Apply() (-expected, + actual) %v", diff)
			}
		})
	}

}

func TestNilRegistryConfig(t *testing.T) {
	testServer, caCert, err := fake.NewFakeConventionServer()
	if err != nil {
		t.Fatalf("unable to create convention server: %v", err)
	}
	testServer.StartTLS()
	defer testServer.Close()

	serverURL, err := url.ParseRequestURI(testServer.URL)
	if err != nil {
		t.Fatalf("this should never happen? %v", err)
	}
	wc := binding.WebhookConfig{
		AuthInfoResolver: webhooktesting.NewAuthenticationInfoResolver(new(int32)),
		ServiceResolver:  fake.NewStubServiceResolver(*serverURL),
	}
	input := binding.Conventions{{
		Name: "my-conventions",
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{
				Namespace: "default",
				Name:      "webhook-test",
			},
			CABundle: caCert,
		},
	}}
	rc := binding.RegistryConfig{}
	namespace := "test-namespace"
	name := "my-template"
	workload := conventionsv1alpha1.PodIntent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: conventionsv1alpha1.PodIntentSpec{
			Template: conventionsv1alpha1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "test-workload",
						Image: fmt.Sprintf("%s/hello@%s", serverURL.Host, HelloDigest),
					}},
				},
			},
		},
	}

	if _, err = input.Apply(context.Background(), &workload, wc, rc); err == nil {
		t.Error("Apply() expected error but got nil")
	}
}

func TestRepositoryConfigWithAdditionalCert(t *testing.T) {
	testServer, caCert, err := fake.NewFakeConventionServer()
	if err != nil {
		t.Fatalf("unable to create convention server: %v", err)
	}
	testServer.StartTLS()
	defer testServer.Close()

	serverURL, err := url.ParseRequestURI(testServer.URL)
	if err != nil {
		t.Fatalf("this should never happen? %v", err)
	}
	wc := binding.WebhookConfig{
		AuthInfoResolver: webhooktesting.NewAuthenticationInfoResolver(new(int32)),
		ServiceResolver:  fake.NewStubServiceResolver(*serverURL),
	}

	dir, err := os.MkdirTemp(os.TempDir(), "ggcr-cache")
	if err != nil {
		t.Fatalf("Unable to create temp dir %v", err)
	}
	defer os.RemoveAll(dir)

	testCache := cache.NewFilesystemCache(dir)
	kc, err := k8schain.NewNoClient(context.Background())
	if err != nil {
		t.Fatalf("Unable to create k8s auth chain %v", err)
	}

	rs, err := registry.TLS("localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()

	rgUrl, err := url.Parse(rs.URL)
	if err != nil {
		t.Fatal(err)
	}

	image, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("Unable to make image: %v", err)
	}

	imageDigest, err := image.Digest()
	if err != nil {
		t.Fatalf("Unable to get image digest: %v", err)
	}
	digestedImage, err := name.NewDigest(rgUrl.Host + "/test@" + imageDigest.String())

	if err != nil {
		t.Fatalf("Unable to parse digest: %v", err)
	}
	if err := remote.Write(digestedImage, image, remote.WithTransport(rs.Client().Transport)); err != nil {
		t.Fatalf("Unable to push image to remote: %s", err)
	}

	cert, err := os.CreateTemp("", "cutomCA")
	if err != nil {
		t.Fatalf("Unable to create temp dir %v", err)
	}
	defer os.RemoveAll(cert.Name())

	if err := pem.Encode(cert, &pem.Block{Type: "CERTIFICATE", Bytes: rs.Certificate().Raw}); err != nil {
		t.Fatalf("Unable to parse certificate %v", err)
	}

	rc := binding.RegistryConfig{Keys: kc, Cache: testCache, CACertPath: cert.Name()}
	namespace := "test-namespace"
	name := "my-template"
	tests := []struct {
		name       string
		convetions []binding.Convention
		workload   *conventionsv1alpha1.PodIntent
		expects    *corev1.PodTemplateSpec
		shouldErr  bool
	}{{
		name: "image registry with customCA",
		convetions: []binding.Convention{{
			Name: "my-conventions",
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service: &admissionregistrationv1.ServiceReference{
					Namespace: "default",
					Name:      "webhook-test",
					// Path:      rtesting.StringPtr("withcustomCA"),
				},
				CABundle: caCert,
			},
		}},
		workload: &conventionsv1alpha1.PodIntent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: conventionsv1alpha1.PodIntentSpec{
				Template: conventionsv1alpha1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "test-workload-main",
							Image: digestedImage.Name(),
						}},
					},
				},
			},
		},
		expects: &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"conventions.carto.run/applied-conventions": "my-conventions/test-convention/default-label"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-workload-main",
						Image: digestedImage.String(),
					},
					{
						Name:  "test-workload",
						Image: "ubuntu",
						Env: []corev1.EnvVar{
							{
								Name:  "KEY",
								Value: "VALUE",
							},
						},
					},
				},
			},
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var input binding.Conventions
			input = append(input, test.convetions...)
			updatedSpec, err := input.Apply(context.Background(), test.workload, wc, rc)
			if (err != nil) != test.shouldErr {
				t.Errorf("Apply() error = %v, ExpectErr %v", err, test.shouldErr)
			}
			if diff := cmp.Diff(test.expects, updatedSpec); diff != "" {
				t.Errorf("Apply() (-expected, + actual) %v", diff)
			}
		})
	}
}
