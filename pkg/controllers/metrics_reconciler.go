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

package controllers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
)

const (
	podIntentsCountMetricPrefix           = "podintents_count"
	podIntentsReadyMetricPrefix           = "podintents_ready"
	podIntentsOwnerMetricPrefix           = "podintents_owner"
	clusterPodConventionNamesMetricPrefix = "clusterpodconventions_names"
)

// MetricsReconciler reconciles workload intent, cluster convention objects
type MetricsReconciler struct {
	client.Client
	Namespace string
	Name      string
}

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=conventions.carto.run,resources=podintents,verbs=get;list;watch
// +kubebuilder:rbac:groups=conventions.carto.run,resources=clusterpodconventions,verbs=get;list;watch

func (r *MetricsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx).WithName("MetricsReconciler")
	ctx = logr.NewContext(ctx, log)

	if req.Namespace != r.Namespace || req.Name != r.Name {
		// ignore other configmaps, should never get here
		return ctrl.Result{}, nil
	}

	var configMap corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &configMap); err != nil && !apierrs.IsNotFound(err) {
		log.Error(err, "unable to fetch ConfigMap")
		return ctrl.Result{}, err
	}
	if configMap.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, nil
	}
	return r.reconcile(ctx, &configMap)
}

func (r *MetricsReconciler) reconcile(ctx context.Context, configMap *corev1.ConfigMap) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)

	var intents conventionsv1alpha1.PodIntentList
	if err := r.List(ctx, &intents); err != nil {
		log.Error(err, "Failed to get PodIntents", "configmap", configMap)
		return ctrl.Result{}, err
	}

	var clusterSources conventionsv1alpha1.ClusterPodConventionList
	if err := r.List(ctx, &clusterSources); err != nil {
		log.Error(err, "Failed to get ClusterPodConventions", "configmap", configMap)
		return ctrl.Result{}, err
	}

	if configMap.Name == "" {
		configMap, err := r.createConfigMap(ctx, buildConfigMapData(intents.Items, clusterSources.Items))
		if err != nil {
			log.Error(err, "Failed to create ConfigMap", "configmap", configMap)
			return ctrl.Result{}, err
		}
	} else {
		configMap, err := r.reconcileConfigMap(ctx, configMap, buildConfigMapData(intents.Items, clusterSources.Items))
		if err != nil {
			log.Error(err, "Failed to reconcile ConfigMap", "configmap", configMap)
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func buildConfigMapData(podIntents []conventionsv1alpha1.PodIntent, clusterPodConventions []conventionsv1alpha1.ClusterPodConvention) map[string]string {
	statusMap := make(map[string]int, 3)
	groupKindMap := make(map[string]int)

	for _, wd := range podIntents {
		ownerRef := metav1.GetControllerOf(&wd.ObjectMeta)
		if ownerRef != nil {
			groupKind := schema.FromAPIVersionAndKind(ownerRef.APIVersion, ownerRef.Kind).GroupKind().String()
			groupKindMap[groupKind] = groupKindMap[groupKind] + 1
		}
		readyCond := wd.Status.GetCondition(conventionsv1alpha1.PodIntentConditionReady)
		status := "unknown"
		if readyCond != nil {
			if readyCond.Status == metav1.ConditionTrue {
				status = "true"
			}
			if readyCond.Status == metav1.ConditionFalse {
				status = "false"
			}
		}
		statusMap[status] = statusMap[status] + 1
	}
	metricsConfigMap := make(map[string]string)
	for k, v := range statusMap {
		metricsConfigMap[fmt.Sprintf("%s_%s_count", podIntentsReadyMetricPrefix, k)] = strconv.Itoa(v)
	}
	for k, v := range groupKindMap {
		metricsConfigMap[fmt.Sprintf("%s_%s_count", podIntentsOwnerMetricPrefix, k)] = strconv.Itoa(v)
	}
	metricsConfigMap[podIntentsCountMetricPrefix] = strconv.Itoa(len(podIntents))

	var conventionNames []string
	for _, os := range clusterPodConventions {
		conventionNames = append(conventionNames, os.Name)
	}
	sort.Strings(conventionNames)
	metricsConfigMap[clusterPodConventionNamesMetricPrefix] = strings.Join(conventionNames, "\n")
	return metricsConfigMap
}

func (r *MetricsReconciler) reconcileConfigMap(ctx context.Context, existingConfigMap *corev1.ConfigMap, configMapContents map[string]string) (*corev1.ConfigMap, error) {
	log := logr.FromContextOrDiscard(ctx)

	configMap := existingConfigMap.DeepCopy()
	configMap.Data = configMapContents

	if configMapSemanticEquals(configMap, existingConfigMap) {
		return existingConfigMap, nil
	}
	log.Info("reconciling builders configmap", "diff", cmp.Diff(existingConfigMap.Data, configMap.Data))
	return configMap, r.Update(ctx, configMap)
}

func (r *MetricsReconciler) createConfigMap(ctx context.Context, configMapContents map[string]string) (*corev1.ConfigMap, error) {
	log := logr.FromContextOrDiscard(ctx)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Name,
			Namespace: r.Namespace,
		},
		Data: configMapContents,
	}
	log.Info("creating builders configmap", "data", configMap.Data)
	return configMap, r.Create(ctx, configMap)
}

func configMapSemanticEquals(desiredConfigMap, configMap *corev1.ConfigMap) bool {
	return equality.Semantic.DeepEqual(desiredConfigMap.Data, configMap.Data)
}

func (r *MetricsReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	enqueueConfigMap := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, a client.Object) []reconcile.Request {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: r.Namespace,
						Name:      r.Name,
					},
				},
			}
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				cm, ok := e.Object.(*corev1.ConfigMap)
				if !ok {
					// not a configmap, allow
					return true
				}
				// filter configmap accounts to only be builders in the system namespace
				return cm.Namespace == r.Namespace && cm.Name == r.Name
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				cm, ok := e.ObjectNew.(*corev1.ConfigMap)
				if !ok {
					// not a configmap, allow
					return true
				}
				// filter configmap accounts to only be builders in the system namespace
				return cm.Namespace == r.Namespace && cm.Name == r.Name
			},
		}).
		Watches(&conventionsv1alpha1.ClusterPodConvention{}, enqueueConfigMap).
		Watches(&conventionsv1alpha1.PodIntent{}, enqueueConfigMap).
		Complete(r)
}
