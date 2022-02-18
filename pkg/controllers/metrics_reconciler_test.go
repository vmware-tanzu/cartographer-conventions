/*
Copyright 2021 VMware Inc.

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
	"testing"
	"time"

	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	"github.com/vmware-labs/reconciler-runtime/reconcilers"
	rtesting "github.com/vmware-labs/reconciler-runtime/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
	controllers "github.com/vmware-tanzu/cartographer-conventions/pkg/controllers"
	dieconventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/dies/conventions/v1alpha1"
)

func TestMetricsReconciler(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "metrics-builders"
	sname := "test-convention"
	sanotherName := "test-another-convention"
	dname := "test-intent"
	anotherDname := "test-another-intent"
	testKey := types.NamespacedName{Namespace: testNamespace, Name: testName}

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = conventionsv1alpha1.AddToScheme(scheme)

	convention := dieconventionsv1alpha1.ClusterPodConventionBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(sname)
		})
	anotherConvention := dieconventionsv1alpha1.ClusterPodConventionBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Name(sanotherName)
		})

	intent := dieconventionsv1alpha1.PodIntentBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(dname)
		})
	anotherIntent := dieconventionsv1alpha1.PodIntentBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(anotherDname)
		})

	testMetrics := diecorev1.ConfigMapBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		})

	secret := diecorev1.SecretBlank.
		MetadataDie(func(d *diemetav1.ObjectMetaDie) {
			d.Namespace(testNamespace)
			d.Name(testName)
		})

	now := metav1.NewTime(time.Now())

	podIntentConditionReady := diemetav1.ConditionBlank.Type(conventionsv1alpha1.PodIntentConditionReady)

	rts := rtesting.ReconcilerTestSuite{
		{
			Name: "builders configmap does not exist",
			Key:  types.NamespacedName{Namespace: testNamespace, Name: "wrong-cm"},
		},
		{
			Name:           "builders configmap with no resources",
			Key:            testKey,
			ExpectedResult: ctrl.Result{},
			ExpectCreates: []client.Object{
				testMetrics.
					AddData("clusterpodconventions_names", "").
					AddData("podintents_count", "0"),
			},
		},
		{
			Name: "builders configmap with exiting resources",
			Key:  testKey,
			GivenObjects: []client.Object{
				testMetrics.
					AddData("clusterpodconventions_names", "").
					AddData("podintents_count", "0"),
			},
			ExpectedResult: ctrl.Result{},
		},
		{
			Name:      "get configmap error",
			Key:       testKey,
			ShouldErr: true,
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("get", "ConfigMap"),
			},
			ExpectedResult: ctrl.Result{},
		},
		{
			Name:      "create configmap error",
			Key:       testKey,
			ShouldErr: true,
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("create", "ConfigMap"),
			},
			ExpectCreates: []client.Object{
				testMetrics.
					AddData("clusterpodconventions_names", "").
					AddData("podintents_count", "0"),
			},
			ExpectedResult: ctrl.Result{},
		},
		{
			Name:      "update configmap error",
			Key:       testKey,
			ShouldErr: true,
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("update", "ConfigMap"),
			},
			GivenObjects: []client.Object{
				testMetrics,
			},
			ExpectUpdates: []client.Object{
				testMetrics.
					AddData("clusterpodconventions_names", "").
					AddData("podintents_count", "0"),
			},
			ExpectedResult: ctrl.Result{},
		},
		{
			Name:      "list cluster sources error",
			Key:       testKey,
			ShouldErr: true,
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("list", "ClusterPodConventionList"),
			},
			GivenObjects: []client.Object{
				convention,
				anotherConvention,
			},
			ExpectedResult: ctrl.Result{},
		},
		{
			Name:      "list intent resources error",
			Key:       testKey,
			ShouldErr: true,
			WithReactors: []rtesting.ReactionFunc{
				rtesting.InduceFailure("list", "PodIntentList"),
			},
			GivenObjects: []client.Object{
				intent,
			},
			ExpectedResult: ctrl.Result{},
		},
		{
			Name: "configmap with delete timestamp",
			Key:  testKey,
			GivenObjects: []client.Object{
				testMetrics.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.DeletionTimestamp(&now)
					}),
			},
		},
		{
			Name: "builders configmap with intent with different status",
			Key:  testKey,
			GivenObjects: []client.Object{
				intent.
					StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
						d.ConditionsDie(
							podIntentConditionReady.Status(metav1.ConditionFalse),
						)
					}),
				anotherIntent.
					StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
						d.ConditionsDie(
							podIntentConditionReady.Status(metav1.ConditionUnknown),
						)
					}),
			},
			ExpectedResult: ctrl.Result{},
			ExpectCreates: []client.Object{
				testMetrics.
					AddData("clusterpodconventions_names", "").
					AddData("podintents_count", "2").
					AddData("podintents_ready_unknown_count", "1").
					AddData("podintents_ready_false_count", "1"),
			},
		},
		{
			Name: "builders configmap with intent resources",
			Key:  testKey,
			GivenObjects: []client.Object{
				intent.
					StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
						d.ConditionsDie(
							podIntentConditionReady.Status(metav1.ConditionTrue),
						)
					}),
				anotherIntent.
					StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
						d.ConditionsDie(
							podIntentConditionReady.Status(metav1.ConditionTrue),
						)
					}),
				anotherConvention,
				convention,
			},
			ExpectedResult: ctrl.Result{},
			ExpectCreates: []client.Object{
				testMetrics.
					AddData("podintents_count", "2").
					AddData("podintents_ready_true_count", "2").
					AddData("clusterpodconventions_names", "test-another-convention\ntest-convention"),
			},
		},
		{
			Name: "builders configmap with intent resources owner references",
			Key:  testKey,
			GivenObjects: []client.Object{
				anotherIntent.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
						d.ControlledBy(secret, scheme)
					}),
				intent.
					MetadataDie(func(d *diemetav1.ObjectMetaDie) {
						d.CreationTimestamp(now)
						d.ControlledBy(secret, scheme)
					}).
					StatusDie(func(d *dieconventionsv1alpha1.PodIntentStatusDie) {
						d.ConditionsDie(
							podIntentConditionReady.Status(metav1.ConditionTrue),
						)
					}),
			},
			ExpectedResult: ctrl.Result{},
			ExpectCreates: []client.Object{
				testMetrics.
					AddData("clusterpodconventions_names", "").
					AddData("podintents_count", "2").
					AddData("podintents_ready_unknown_count", "1").
					AddData("podintents_ready_true_count", "1").
					AddData("podintents_owner_Secret_count", "2"),
			},
		},
	}
	rts.Run(t, scheme, func(t *testing.T, rtc *rtesting.ReconcilerTestCase, c reconcilers.Config) reconcile.Reconciler {
		return &controllers.MetricsReconciler{
			Client:    c.Client,
			Log:       c.Log,
			Namespace: testNamespace,
			Name:      testName,
		}
	})
}
