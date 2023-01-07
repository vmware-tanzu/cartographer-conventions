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

package controllers_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	dieadmissionregistrationv1 "dies.dev/apis/admissionregistration/v1"
	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	webhooktesting "k8s.io/apiserver/pkg/admission/plugin/webhook/testing"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
	certmanagerv1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/thirdparty/cert-manager/v1"
	"github.com/vmware-tanzu/cartographer-conventions/pkg/binding"
	"github.com/vmware-tanzu/cartographer-conventions/pkg/binding/fake"
	controllers "github.com/vmware-tanzu/cartographer-conventions/pkg/controllers"
	diecertmanagerv1 "github.com/vmware-tanzu/cartographer-conventions/pkg/dies/cert-manager/v1"
	dieconventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/dies/conventions/v1alpha1"
)

const (
	defaultSAName = "default"
)

const HelloDigest = "sha256:fede69b4ce95775cc92af3605555c2078b9b6d5eb3fb45d2d67fd6ac7a0209b7"

func intPtr(s int32) *int32 { return &s }

func TestPodIntentReconciler(t *testing.T) {
	conventionServer, _, err := fake.NewFakeConventionServer()
	if err != nil {
		t.Fatalf("unable to create fake convnetion server: %v", err)
	}
	conventionServer.StartTLS()
	defer conventionServer.Close()
	serverURL, err := url.ParseRequestURI(conventionServer.URL)
	if err != nil {
		t.Fatalf("this should never happen? %v", err)
	}
	wc := binding.WebhookConfig{
		AuthInfoResolver: webhooktesting.NewAuthenticationInfoResolver(new(int32)),
		ServiceResolver:  fake.NewStubServiceResolver(*serverURL),
	}

	registryServer := httptest.NewServer(registry.New())
	registryUrl, _ := url.Parse(registryServer.URL)

	namespace := "test-namespace"
	name := "my-template"
	secretName := "test-secret"

	key := types.NamespacedName{Namespace: namespace, Name: name}
	image := fmt.Sprintf("%s/%s", registryUrl.Host, "img")

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = conventionsv1alpha1.AddToScheme(scheme)

	dir, err := ioutil.TempDir("", "ggcr-cache")
	if err != nil {
		t.Fatalf("Unable to create temp dir %v", err)
	}

	now := metav1.Now()

	secret := diecorev1.SecretBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(secretName)
		})
	defaultSA := diecorev1.ServiceAccountBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(defaultSAName)
			d.CreationTimestamp(now)
		})

	defer os.RemoveAll(dir)
	testCache := cache.NewFilesystemCache(dir)
	testClient := fakeclient.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaultSAName,
				Namespace: namespace,
			},
		},
	)
	rc := binding.RegistryConfig{Cache: testCache, Client: testClient}

	parent := dieconventionsv1alpha1.PodIntentBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.CreationTimestamp(now)
		})

	rts := rtesting.ReconcilerTestSuite{{
		Name: "in sync",
		Key:  key,
		GivenObjects: []client.Object{
			defaultSA,
			secret,
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ImagePullSecretsDie(
						diecorev1.LocalObjectReferenceBlank.Name(secretName),
					)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(image)
							})
						})
					})
				}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionTrue).
							Reason("Applied"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionTrue).
							Reason("ConventionsApplied"),
					)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(image)
							})
						})
					})
				}),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(secret, parent, scheme),
			rtesting.NewTrackRequest(defaultSA, parent, scheme),
		},
	}}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return controllers.PodIntentReconciler(c, wc, rc)
	})
}

func TestBuildRegistryConfig(t *testing.T) {
	conventionServer, _, err := fake.NewFakeConventionServer()
	if err != nil {
		t.Fatalf("unable to create fake convention server: %v", err)
	}
	conventionServer.StartTLS()
	defer conventionServer.Close()
	serverURL, err := url.ParseRequestURI(conventionServer.URL)
	if err != nil {
		t.Fatalf("this should never happen? %v", err)
	}
	wc := binding.WebhookConfig{
		AuthInfoResolver: webhooktesting.NewAuthenticationInfoResolver(new(int32)),
		ServiceResolver:  fake.NewStubServiceResolver(*serverURL),
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

	namespace := "test-namespace"
	name := "my-template"
	secretName := "test-secret"
	saWithSecret := "test-sa-with-secret"

	key := types.NamespacedName{Namespace: namespace, Name: name}

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = conventionsv1alpha1.AddToScheme(scheme)

	dir, err := ioutil.TempDir("", "ggcr-cache")
	if err != nil {
		t.Fatalf("Unable to create temp dir %v", err)
	}
	defer os.RemoveAll(dir)
	testCache := cache.NewFilesystemCache(dir)
	testClient := fakeclient.NewSimpleClientset(
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaultSAName,
				Namespace: namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saWithSecret,
				Namespace: namespace,
			},
			ImagePullSecrets: []corev1.LocalObjectReference{{
				Name: secretName,
			}},
		},
	)

	rc := binding.RegistryConfig{
		Cache:  testCache,
		Client: testClient,
	}

	now := metav1.Now()

	parent := dieconventionsv1alpha1.PodIntentBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.CreationTimestamp(now)
		})
	secret := diecorev1.SecretBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(secretName)
			d.CreationTimestamp(now)
		})
	absentSecret := diecorev1.SecretBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name("wrong-secret")
			d.CreationTimestamp(now)
		})
	sa := diecorev1.ServiceAccountBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(saWithSecret)
			d.CreationTimestamp(now)
		}).
		ImagePullSecretsDie(
			diecorev1.LocalObjectReferenceBlank.Name(secretName),
		)
	defaultSA := diecorev1.ServiceAccountBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(defaultSAName)
			d.CreationTimestamp(now)
		})

	rts := rtesting.ReconcilerTestSuite{{
		Name: "image pull secret",
		Key:  key,
		GivenObjects: []client.Object{
			defaultSA,
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ImagePullSecretsDie(
						diecorev1.LocalObjectReferenceBlank.Name("test-secret"),
					)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(secret, parent, scheme),
			rtesting.NewTrackRequest(defaultSA, parent, scheme),
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(parent, scheme, corev1.EventTypeNormal, "StatusUpdated", `Updated status`),
		},
		ExpectStatusUpdates: []client.Object{
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ServiceAccountName(saWithSecret)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionTrue).
							Reason("Applied"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionTrue).
							Reason("ConventionsApplied"),
					)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}),
		},
	}, {
		Name: "service account with image pull secret",
		Key:  key,
		GivenObjects: []client.Object{
			sa,
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ServiceAccountName(saWithSecret)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}),
		},
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(parent, scheme, corev1.EventTypeNormal, "StatusUpdated", `Updated status`),
		},
		ExpectStatusUpdates: []client.Object{
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ServiceAccountName(saWithSecret)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionTrue).
							Reason("Applied"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionTrue).
							Reason("ConventionsApplied"),
					)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(sa, parent, scheme),
			rtesting.NewTrackRequest(secret, parent, scheme),
		},
	}, {
		Name: "ServiceAccount not present in namespace",
		Key:  key,
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(parent, scheme, corev1.EventTypeNormal, "StatusUpdated", `Updated status`),
		},
		GivenObjects: []client.Object{
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ServiceAccountName("wrong-sa")
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}),
		},
		ExpectStatusUpdates: []client.Object{
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ImagePullSecretsDie(
						diecorev1.LocalObjectReferenceBlank.Name("wrong-secret"),
					)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionFalse).
							Reason("ImageResolutionFailed").
							Message("failed to authenticate: serviceaccounts \"wrong-sa\" not found"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionFalse).
							Reason("ImageResolutionFailed").
							Message("failed to authenticate: serviceaccounts \"wrong-sa\" not found"),
					)
				}),
		},
	}, {
		Name: "ServiceAccount not present in api reader(unlikely)",
		Key:  key,
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(parent, scheme, corev1.EventTypeNormal, "StatusUpdated", `Updated status`),
		},
		GivenObjects: []client.Object{
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ServiceAccountName(saWithSecret)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}),
		},
		ExpectStatusUpdates: []client.Object{
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ServiceAccountName(saWithSecret)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionFalse).
							Reason("ImageResolutionFailed").
							Message("failed to authenticate: serviceaccounts \"test-sa-with-secret\" not found"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionFalse).
							Reason("ImageResolutionFailed").
							Message("failed to authenticate: serviceaccounts \"test-sa-with-secret\" not found"),
					)
				}),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(sa, parent, scheme),
		},
	}, {
		Name: "secret not present in namespace",
		Key:  key,
		ExpectEvents: []rtesting.Event{
			rtesting.NewEvent(parent, scheme, corev1.EventTypeNormal, "StatusUpdated", `Updated status`),
		},
		ExpectStatusUpdates: []client.Object{
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ImagePullSecretsDie(
						diecorev1.LocalObjectReferenceBlank.Name("wrong-secret"),
					)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionFalse).
							Reason("ImageResolutionFailed").
							Message("failed to authenticate: secrets \"wrong-secret\" not found"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionFalse).
							Reason("ImageResolutionFailed").
							Message("failed to authenticate: secrets \"wrong-secret\" not found"),
					)
				}),
		},
		GivenObjects: []client.Object{
			parent.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.ImagePullSecretsDie(
						diecorev1.LocalObjectReferenceBlank.Name("wrong-secret"),
					)
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("workload", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
							})
						})
					})
				}),
		},
		ExpectTracks: []rtesting.TrackRequest{
			rtesting.NewTrackRequest(absentSecret, parent, scheme),
		},
	}}
	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return controllers.PodIntentReconciler(c, wc, rc)
	})
}

func TestResolveConventions(t *testing.T) {
	testName := "test-convention"
	anotherTestName := "another-test-convention"
	url := "https://example.com/"
	namespace := "test-namespace"
	cname := "my-cert"
	name := "my-template"

	now := metav1.Now()

	// the parent type doesn't matter as this reconciler doesn't use the parent
	parent := dieconventionsv1alpha1.PodIntentBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
		})

	certReq := diecertmanagerv1.CertificateRequestBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.CreationTimestamp(now)
		}).
		StatusDie(func(d *diecertmanagerv1.CertificateRequestStatusDie) {
			d.CA(BadCACert)
			d.ConditionsDie(
				diecertmanagerv1.CertificateRequestConditionReadyBlank.Status(metav1.ConditionTrue),
			)
		})

	serviceReference := &admissionregistrationv1.ServiceReference{
		Namespace: "default",
		Name:      "convention-server",
		Port:      intPtr(443),
	}
	testConvention := dieconventionsv1alpha1.ClusterPodConventionBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(testName)
		})

	anotherTestConvention := dieconventionsv1alpha1.ClusterPodConventionBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(anotherTestName)
		})

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = conventionsv1alpha1.AddToScheme(scheme)
	_ = certmanagerv1.AddToScheme(scheme)

	rts := rtesting.SubReconcilerTestSuite{
		{
			Name:     "stash convention",
			Resource: parent,
			GivenObjects: []client.Object{
				certReq.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.AddAnnotation("cert-manager.io/certificate-name", cname)
					}),
				testConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
								d.Service(serviceReference)
							})
							d.CertificateDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookCertificateDie) {
								d.Namespace(namespace)
								d.Name(cname)
							})
						})
					}),
				anotherTestConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfig(admissionregistrationv1.WebhookClientConfig{URL: &url})
						})
					}),
			},
			ExpectResource: parent,
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ConventionsStashKey: []binding.Convention{
					{
						Name:         anotherTestName,
						Priority:     conventionsv1alpha1.NormalPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{URL: &url},
					},
					{
						Name:         testName,
						Priority:     conventionsv1alpha1.NormalPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: serviceReference, CABundle: BadCACert},
					}},
			},
		}, {
			Name:     "error loading conventions",
			Resource: parent,
			GivenObjects: []client.Object{
				testConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
								d.Service(serviceReference)
							})
						})
					}),
				anotherTestConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfig(admissionregistrationv1.WebhookClientConfig{URL: &url})
						})
					}),
			},
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("list", "clusterpodconventionlist"),
			},
			ShouldErr:      true,
			ExpectResource: parent,
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ConventionsStashKey: nil,
			},
		}, {
			Name:     "use three most recent ready CAs",
			Resource: parent,
			GivenObjects: []client.Object{
				certReq.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Namespace(namespace)
						d.Name(name + "-1")
						d.CreationTimestamp(metav1.NewTime(time.UnixMilli(1000)))
						d.AddAnnotation("cert-manager.io/certificate-name", cname)
					}).
					StatusDie(func(d *diecertmanagerv1.CertificateRequestStatusDie) {
						d.CA([]byte("1\n"))
					}),
				certReq.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Namespace(namespace)
						d.Name(name + "-2")
						d.CreationTimestamp(metav1.NewTime(time.UnixMilli(2000)))
						d.AddAnnotation("cert-manager.io/certificate-name", cname)
					}).
					StatusDie(func(d *diecertmanagerv1.CertificateRequestStatusDie) {
						d.CA([]byte("2\n"))
					}),
				certReq.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Namespace(namespace)
						d.Name(name + "-3")
						d.CreationTimestamp(metav1.NewTime(time.UnixMilli(3000)))
						d.AddAnnotation("cert-manager.io/certificate-name", cname)
					}).
					StatusDie(func(d *diecertmanagerv1.CertificateRequestStatusDie) {
						d.CA([]byte("3\n"))
					}),
				certReq.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Namespace(namespace)
						d.Name(name + "-4")
						d.CreationTimestamp(metav1.NewTime(time.UnixMilli(4000)))
						d.AddAnnotation("cert-manager.io/certificate-name", cname)
					}).
					StatusDie(func(d *diecertmanagerv1.CertificateRequestStatusDie) {
						d.CA([]byte("4\n"))
					}),
				certReq.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.Namespace(namespace)
						d.Name(name + "-5")
						d.CreationTimestamp(metav1.NewTime(time.UnixMilli(5000)))
						d.AddAnnotation("cert-manager.io/certificate-name", cname)
					}).
					StatusDie(func(d *diecertmanagerv1.CertificateRequestStatusDie) {
						d.CA([]byte("5\n"))
					}),
				testConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
								d.Service(serviceReference)
							})
							d.CertificateDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookCertificateDie) {
								d.Namespace(namespace)
								d.Name(cname)
							})
						})
					}),
				anotherTestConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfig(admissionregistrationv1.WebhookClientConfig{URL: &url})
						})
					}),
			},
			ExpectResource: parent,
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ConventionsStashKey: []binding.Convention{
					{
						Name:         anotherTestName,
						Priority:     conventionsv1alpha1.NormalPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{URL: &url},
					},
					{
						Name:         testName,
						Priority:     conventionsv1alpha1.NormalPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: serviceReference, CABundle: []byte("5\n4\n3\n")},
					}},
			},
		}, {
			Name:     "cert request not present",
			Resource: parent,
			GivenObjects: []client.Object{
				testConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
								d.Service(serviceReference)
							})
							d.CertificateDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookCertificateDie) {
								d.Namespace("ns")
								d.Name("wrong-ca")
							})
						})
					}),
			},
			ExpectResource: parent.
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionFalse).
							Reason("CABundleResolutionFailed").
							Message(`failed to authenticate: unable to find valid certificaterequests for certificate "ns/wrong-ca" configured in convention "test-convention"`),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionFalse).
							Reason("CABundleResolutionFailed").
							Message(`failed to authenticate: unable to find valid certificaterequests for certificate "ns/wrong-ca" configured in convention "test-convention"`),
					)
				}),
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ConventionsStashKey: nil,
			},
		}, {
			Name:     "cert request not owned by cert",
			Resource: parent,
			GivenObjects: []client.Object{
				certReq.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.AddAnnotation("cert-manager.io/certificate-name", "some-other-cert")
					}),
				testConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
								d.Service(serviceReference)
							})
							d.CertificateDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookCertificateDie) {
								d.Namespace(namespace)
								d.Name(cname)
							})
						})
					}),
			},
			ExpectResource: parent.
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionFalse).
							Reason("CABundleResolutionFailed").
							Message(`failed to authenticate: unable to find valid certificaterequests for certificate "test-namespace/my-cert" configured in convention "test-convention"`),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionFalse).
							Reason("CABundleResolutionFailed").
							Message(`failed to authenticate: unable to find valid certificaterequests for certificate "test-namespace/my-cert" configured in convention "test-convention"`),
					)
				}),
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ConventionsStashKey: nil,
			},
		}, {
			Name:     "cert request not ready",
			Resource: parent,
			GivenObjects: []client.Object{
				certReq.
					StatusDie(func(d *diecertmanagerv1.CertificateRequestStatusDie) {
						d.ConditionsDie(
							diecertmanagerv1.CertificateRequestConditionReadyBlank.Status(metav1.ConditionUnknown),
						)
					}),
				testConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
								d.Service(serviceReference)
							})
							d.CertificateDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookCertificateDie) {
								d.Namespace(namespace)
								d.Name(cname)
							})
						})
					}),
			},
			ExpectResource: parent.
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionFalse).
							Reason("CABundleResolutionFailed").
							Message(`failed to authenticate: unable to find valid certificaterequests for certificate "test-namespace/my-cert" configured in convention "test-convention"`),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionFalse).
							Reason("CABundleResolutionFailed").
							Message(`failed to authenticate: unable to find valid certificaterequests for certificate "test-namespace/my-cert" configured in convention "test-convention"`),
					)
				}),
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ConventionsStashKey: nil,
			},
		}, {
			Name:     "cert request no ca",
			Resource: parent,
			GivenObjects: []client.Object{
				certReq.
					StatusDie(func(d *diecertmanagerv1.CertificateRequestStatusDie) {
						r := d.DieRelease()
						r.CA = nil
						d.DieFeed(r)
					}),
				testConvention.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
					}).
					SpecDie(func(d *dieconventionsv1alpha1.ClusterPodConventionSpecDie) {
						d.WebookDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookDie) {
							d.ClientConfigDie(func(d *dieadmissionregistrationv1.WebhookClientConfigDie) {
								d.Service(serviceReference)
							})
							d.CertificateDie(func(d *dieconventionsv1alpha1.ClusterPodConventionWebhookCertificateDie) {
								d.Namespace(namespace)
								d.Name(cname)
							})
						})
					}),
			},
			ExpectResource: parent.
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionFalse).
							Reason("CABundleResolutionFailed").
							Message(`failed to authenticate: unable to find valid certificaterequests for certificate "test-namespace/my-cert" configured in convention "test-convention"`),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionFalse).
							Reason("CABundleResolutionFailed").
							Message(`failed to authenticate: unable to find valid certificaterequests for certificate "test-namespace/my-cert" configured in convention "test-convention"`),
					)
				}),
			ExpectStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ConventionsStashKey: nil,
			},
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.ResolveConventions()
	})
}

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

func TestApplyConventionsReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test-intent"
	testConventions := "my-conventions"

	// using Workload, but any compatible type will work
	workload := dieconventionsv1alpha1.PodIntentBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		}).
		StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
			d.ConditionsDie(
				dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.Status(metav1.ConditionUnknown),
			)
		})

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = conventionsv1alpha1.AddToScheme(scheme)

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
	kc, err := k8schain.NewNoClient(context.Background())
	if err != nil {
		t.Fatalf("Unable to create k8s auth chain %v", err)
	}
	dir, err := ioutil.TempDir("", "ggcr-cache")
	if err != nil {
		t.Fatalf("Unable to create temp dir %v", err)
	}
	defer os.RemoveAll(dir)

	testCache := cache.NewFilesystemCache(dir)

	rc := binding.RegistryConfig{Keys: kc, Cache: testCache}

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

	rts := rtesting.SubReconcilerTestSuite{
		{
			Name:     "resolved from service",
			Resource: workload,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.RegistryConfigKey: rc,
				controllers.ConventionsStashKey: []binding.Convention{
					{
						Name:     testConventions,
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
			},
			ExpectResource: workload.
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.AddAnnotation(conventionsv1alpha1.AppliedConventionsAnnotationKey, "my-conventions/test-convention/default-label")
						})
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("test-workload", func(d *diecorev1.ContainerDie) {
								d.Image("ubuntu")
								d.EnvDie("KEY", func(d *diecorev1.EnvVarDie) {
									d.Value("VALUE")
								})
							})
						})
					})
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionTrue).
							Reason("Applied"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionTrue).
							Reason("ConventionsApplied"),
					)
				}),
		},
		{
			Name: "selector target and matcher defined matcheslabels in podTemplateSpec values",
			Resource: workload.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.AddLabel("foo", "bar")
							d.AddLabel("zoo", "zebra")
						})
					})
				}).
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.AddLabel("environment", "development")
				}),
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.RegistryConfigKey: rc,
				controllers.ConventionsStashKey: []binding.Convention{
					{
						Name:           testConventions,
						SelectorTarget: "PodTemplateSpec",
						Selectors: []metav1.LabelSelector{{
							MatchLabels: map[string]string{"foo": "bar"},
						}},
						Priority: conventionsv1alpha1.EarlyPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Namespace: "default",
								Name:      "webhook-test",
								Path:      rtesting.StringPtr(fmt.Sprintf("hellosidecar;host=%s", registryUrl.Host)),
							},
							CABundle: caCert,
						},
					},
					{
						Name:           testConventions,
						SelectorTarget: "PodIntent",
						Selectors: []metav1.LabelSelector{{
							MatchLabels: map[string]string{"non-matching": "development"},
						}},
						Priority: conventionsv1alpha1.EarlyPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Namespace: "default",
								Name:      "non-matching-webhook",
								Path:      rtesting.StringPtr(fmt.Sprintf("hellosidecar;host=%s", registryUrl.Host)),
							},
							CABundle: caCert,
						},
					},
				},
			},
			ExpectResource: workload.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.AddLabel("foo", "bar")
							d.AddLabel("zoo", "zebra")
						})
					})
				}).MetadataDie(func(d *diemetav1.ObjectMetaDie) {
				d.AddLabel("environment", "development")
			}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.AddLabel("foo", "bar")
							d.AddLabel("zoo", "zebra")
							d.AddAnnotation("conventions.carto.run/applied-conventions", "my-conventions/path/hellosidecar")
						})
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("hellosidecar", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello", registryUrl.Host))
								d.Command("/bin/sleep", "100")
							})
						})
					})
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionTrue).
							Reason("Applied"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionTrue).
							Reason("ConventionsApplied"),
					)
				}),
		},
		{
			Name: "multiple selector targets and matching labels exist on specified target",
			Resource: workload.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.AddLabel("intentselector", "true")
				}).SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
				d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
					d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.AddLabel("zoo", "zebra")
					})
				})
			}),
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.RegistryConfigKey: rc,
				controllers.ConventionsStashKey: []binding.Convention{
					{
						Name:           "zoo-conventions",
						SelectorTarget: "PodIntent",
						Selectors: []metav1.LabelSelector{{
							MatchLabels: map[string]string{"intentselector": "true"},
						}},
						Priority: conventionsv1alpha1.NormalPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Namespace: "default",
								Name:      "webhook-test",
							},
							CABundle: caCert,
						},
					},
					{
						Name:           testConventions,
						SelectorTarget: "PodTemplateSpec",
						Selectors: []metav1.LabelSelector{{
							MatchLabels: map[string]string{"foo": "bar"},
						}},
						Priority: conventionsv1alpha1.EarlyPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Namespace: "default",
								Name:      "webhook-test",
								Path:      rtesting.StringPtr(fmt.Sprintf("hellosidecar;host=%s", registryUrl.Host)),
							},
							CABundle: caCert,
						},
					},
					{
						Name:           "mismatch-convention-label",
						SelectorTarget: "PodTemplateSpec",
						Selectors: []metav1.LabelSelector{{
							MatchLabels: map[string]string{"bar": "baz"},
						}},
						Priority: conventionsv1alpha1.NormalPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Namespace: "default",
								Name:      "webhook-test",
								Path:      rtesting.StringPtr("labelonly"),
							},
							CABundle: caCert,
						},
					},
				},
			},
			ExpectResource: workload.
				MetadataDie(func(d *diemetav1.ObjectMetaDie) {
					d.AddLabel("intentselector", "true")
				}).
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.AddLabel("zoo", "zebra")
						})
					})
				}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.AddLabel("zoo", "zebra")
							d.AddAnnotation(conventionsv1alpha1.AppliedConventionsAnnotationKey, "zoo-conventions/test-convention/default-label")
						})
						d.SpecDie(func(d *diecorev1.PodSpecDie) {

							d.ContainerDie("test-workload", func(d *diecorev1.ContainerDie) {
								d.Image("ubuntu")
								d.EnvDie("KEY", func(d *diecorev1.EnvVarDie) {
									d.Value("VALUE")
								})
							})
						})
					})
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionTrue).
							Reason("Applied"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionTrue).
							Reason("ConventionsApplied"),
					)
				}),
		},
		{
			Name:     "apply all conventions if no convnetion matchers are set and no matching labels are available on the pod intent",
			Resource: workload,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.RegistryConfigKey: rc,
				controllers.ConventionsStashKey: []binding.Convention{
					{
						Name:           "zoo-conventions",
						SelectorTarget: "PodTemplateSpec",
						Priority:       conventionsv1alpha1.NormalPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Namespace: "default",
								Name:      "webhook-test",
							},
							CABundle: caCert,
						},
					},
					{
						Name:           testConventions,
						SelectorTarget: "PodTemplateSpec",
						Priority:       conventionsv1alpha1.EarlyPriority,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Namespace: "default",
								Name:      "webhook-test",
								Path:      rtesting.StringPtr(fmt.Sprintf("hellosidecar;host=%s", registryUrl.Host)),
							},
							CABundle: caCert,
						},
					},
				},
			},
			ExpectResource: workload.
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.MetadataDie(func(d *diemetav1.ObjectMetaDie) {
							d.AddAnnotation(conventionsv1alpha1.AppliedConventionsAnnotationKey, "my-conventions/path/hellosidecar\nzoo-conventions/test-convention/default-label")
						})
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("hellosidecar", func(d *diecorev1.ContainerDie) {
								d.Image(fmt.Sprintf("%s/hello:latest@%s", registryUrl.Host, HelloDigest))
								d.Command("/bin/sleep", "100")
							})
							d.ContainerDie("test-workload", func(d *diecorev1.ContainerDie) {
								d.Image("ubuntu")
								d.EnvDie("KEY", func(d *diecorev1.EnvVarDie) {
									d.Value("VALUE")
								})
							})
						})
					})
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionTrue).
							Reason("Applied"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionTrue).
							Reason("ConventionsApplied"),
					)
				}),
		},
		{
			Name:     "bad matching expression",
			Resource: workload,
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.RegistryConfigKey: rc,
				controllers.ConventionsStashKey: []binding.Convention{
					{
						Name: testConventions,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							URL:      &testServer.URL,
							CABundle: caCert,
						},
						Priority: conventionsv1alpha1.NormalPriority,
						Selectors: []metav1.LabelSelector{{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Values:   []string{"elephant", "zebra"},
								Operator: metav1.LabelSelectorOpIn,
							}},
						}},
					},
				},
			},
			ExpectedResult: reconcile.Result{},
			ExpectResource: workload.
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionFalse).
							Reason("LabelSelector").
							Message("filtering conventions failed: converting label selector for clusterPodConvention \"my-conventions\" failed: key: Invalid value: \"\": name part must be non-empty; name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionFalse).
							Reason("LabelSelector").
							Message("filtering conventions failed: converting label selector for clusterPodConvention \"my-conventions\" failed: key: Invalid value: \"\": name part must be non-empty; name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')"),
					)
				}),
		},
		{
			Name: "error applying conventions",
			Resource: workload.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("test-workload", func(d *diecorev1.ContainerDie) {
								d.Image("ubuntu")
							})
						})
					})
				}),
			GivenStashedValues: map[reconcilers.StashKey]interface{}{
				controllers.ConventionsStashKey: []binding.Convention{
					{
						Name:     testConventions,
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
			},
			ExpectedResult: reconcile.Result{Requeue: true},
			ExpectResource: workload.
				SpecDie(func(d *dieconventionsv1alpha1.PodIntentSpecDie) {
					d.TemplateDie(func(d *diecorev1.PodTemplateSpecDie) {
						d.SpecDie(func(d *diecorev1.PodSpecDie) {
							d.ContainerDie("test-workload", func(d *diecorev1.ContainerDie) {
								d.Image("ubuntu")
							})
						})
					})
				}).
				StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
					d.ConditionsDie(
						dieconventionsv1alpha1.PodIntentConditionConventionsAppliedBlank.
							Status(metav1.ConditionFalse).
							Reason("ConventionsApplied").
							Message("fetching metadata for Images failed: image: \"ubuntu\" error: registry config keys are not set"),
						dieconventionsv1alpha1.PodIntentConditionReadyBlank.
							Status(metav1.ConditionFalse).
							Reason("ConventionsApplied").
							Message("fetching metadata for Images failed: image: \"ubuntu\" error: registry config keys are not set"),
					)
				}),
		},
	}

	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.ApplyConventionsReconciler(wc)
	})
}

func TestStashConventions(t *testing.T) {
	ctx := reconcilers.WithStash(context.TODO())
	var expected, actual []binding.Convention

	actual = controllers.RetrieveConventions(ctx)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual) = %v", diff)
	}

	expected = []binding.Convention{
		{Name: "my-conventions"},
	}
	controllers.StashConventions(ctx, expected)
	actual = controllers.RetrieveConventions(ctx)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual) = %v", diff)
	}
}

func TestStashRegistryConfig(t *testing.T) {
	ctx := reconcilers.WithStash(context.TODO())
	var expected, actual binding.RegistryConfig

	actual = controllers.RetrieveRegistryConfig(ctx)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual) = %v", diff)
	}

	expected = binding.RegistryConfig{}

	controllers.StashRegistryConfig(ctx, expected)
	actual = controllers.RetrieveRegistryConfig(ctx)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("(-expected, +actual) = %v", diff)
	}
}

func TestNilClientBuildRegistryConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = conventionsv1alpha1.AddToScheme(scheme)

	namespace := "test-namespace"
	name := "my-template"
	now := metav1.Now()

	parent := dieconventionsv1alpha1.PodIntentBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(namespace)
			d.Name(name)
			d.CreationTimestamp(now)
		})

	rts := rtesting.SubReconcilerTestSuite{{
		Name:      "empty client",
		Resource:  parent,
		ShouldErr: true,
	}}
	rc := binding.RegistryConfig{}
	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.SubReconcilerTestCase, c reconcilers.Config) reconcilers.SubReconciler {
		return controllers.BuildRegistryConfig(rc)
	})
}
