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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	_ metav1.ObjectMetaAccessor = (*PodTemplateSpec)(nil)
	_ metav1.Object             = (*ObjectMeta)(nil)
)

// PodTemplateSpec mirrors corev1.PodTemplateSpec with a simplified ObjectMeta.
// This hacks around an issue with controller-gen where it doesn't generate the
// correct structural scheme that results in all ObjectMeta fields being lost.
// See https://github.com/kubernetes-sigs/controller-tools/issues/448
type PodTemplateSpec struct {
	ObjectMeta `json:"metadata,omitempty"`
	Spec       corev1.PodSpec `json:"spec,omitempty"`
}

func NewPodTemplateSpec(pts *corev1.PodTemplateSpec) *PodTemplateSpec {
	return &PodTemplateSpec{
		ObjectMeta: ObjectMeta{
			Name:         pts.Name,
			GenerateName: pts.GenerateName,
			Namespace:    pts.Namespace,
			Labels:       pts.Labels,
			Annotations:  pts.Annotations,
		},
		Spec: pts.Spec,
	}
}

func (p *PodTemplateSpec) AsPodTemplateSpec() *corev1.PodTemplateSpec {
	if p == nil {
		return nil
	}
	return &corev1.PodTemplateSpec{
		ObjectMeta: p.AsObjectMeta(),
		Spec:       p.Spec,
	}
}

func (p *PodTemplateSpec) GetObjectMeta() metav1.Object {
	if p == nil {
		return nil
	}
	return &p.ObjectMeta
}

type ObjectMeta struct {
	Name         string            `json:"name,omitempty"`
	GenerateName string            `json:"generateName,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
}

func (m *ObjectMeta) AsObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:         m.Name,
		GenerateName: m.GenerateName,
		Namespace:    m.Namespace,
		Labels:       m.Labels,
		Annotations:  m.Annotations,
	}
}

func (m *ObjectMeta) GetNamespace() string                         { return m.Namespace }
func (m *ObjectMeta) SetNamespace(namespace string)                { m.Namespace = namespace }
func (m *ObjectMeta) GetName() string                              { return m.Name }
func (m *ObjectMeta) SetName(name string)                          { m.Name = name }
func (m *ObjectMeta) GetGenerateName() string                      { return m.GenerateName }
func (m *ObjectMeta) SetGenerateName(name string)                  { m.GenerateName = name }
func (m *ObjectMeta) GetLabels() map[string]string                 { return m.Labels }
func (m *ObjectMeta) SetLabels(labels map[string]string)           { m.Labels = labels }
func (m *ObjectMeta) GetAnnotations() map[string]string            { return m.Annotations }
func (m *ObjectMeta) SetAnnotations(annotations map[string]string) { m.Annotations = annotations }

func (m *ObjectMeta) GetUID() types.UID    { panic(fmt.Errorf("GetUID is not implemented")) }
func (m *ObjectMeta) SetUID(uid types.UID) { panic(fmt.Errorf("SetUID is not implemented")) }
func (m *ObjectMeta) GetResourceVersion() string {
	panic(fmt.Errorf("GetResourceVersion is not implemented"))
}
func (m *ObjectMeta) SetResourceVersion(version string) {
	panic(fmt.Errorf("SetResourceVersion is not implemented"))
}
func (m *ObjectMeta) GetGeneration() int64 { panic(fmt.Errorf("GetGeneration is not implemented")) }
func (m *ObjectMeta) SetGeneration(generation int64) {
	panic(fmt.Errorf("SetGeneration is not implemented"))
}
func (m *ObjectMeta) GetSelfLink() string {
	panic(fmt.Errorf("GetSelfLink is not implemented"))
}
func (m *ObjectMeta) SetSelfLink(selfLink string) {
	panic(fmt.Errorf("SetSelfLink is not implemented"))
}
func (m *ObjectMeta) GetCreationTimestamp() metav1.Time {
	panic(fmt.Errorf("GetCreationTimestamp is not implemented"))
}
func (m *ObjectMeta) SetCreationTimestamp(timestamp metav1.Time) {
	panic(fmt.Errorf("SetCreationTimestamp is not implemented"))
}
func (m *ObjectMeta) GetDeletionTimestamp() *metav1.Time {
	panic(fmt.Errorf("GetDeletionTimestamp is not implemented"))
}
func (m *ObjectMeta) SetDeletionTimestamp(timestamp *metav1.Time) {
	panic(fmt.Errorf("SetDeletionTimestamp is not implemented"))
}
func (m *ObjectMeta) GetDeletionGracePeriodSeconds() *int64 {
	panic(fmt.Errorf("GetDeletionGracePeriodSeconds is not implemented"))
}
func (m *ObjectMeta) SetDeletionGracePeriodSeconds(*int64) {
	panic(fmt.Errorf("SetDeletionGracePeriodSeconds is not implemented"))
}
func (m *ObjectMeta) GetFinalizers() []string {
	panic(fmt.Errorf("GetFinalizers is not implemented"))
}
func (m *ObjectMeta) SetFinalizers(finalizers []string) {
	panic(fmt.Errorf("SetFinalizers is not implemented"))
}
func (m *ObjectMeta) GetOwnerReferences() []metav1.OwnerReference {
	panic(fmt.Errorf("GetOwnerReferences is not implemented"))
}
func (m *ObjectMeta) SetOwnerReferences([]metav1.OwnerReference) {
	panic(fmt.Errorf("SetOwnerReferences is not implemented"))
}
func (m *ObjectMeta) GetClusterName() string {
	panic(fmt.Errorf("GetClusterName is not implemented"))
}
func (m *ObjectMeta) SetClusterName(clusterName string) {
	panic(fmt.Errorf("SetClusterName is not implemented"))
}
func (m *ObjectMeta) GetZZZDeprecatedClusterName() string {
	panic(fmt.Errorf("GetZZZDeprecatedClusterName is not implemented"))
}
func (m *ObjectMeta) SetZZZDeprecatedClusterName(clusterName string) {
	panic(fmt.Errorf("SetZZZDeprecatedClusterName is not implemented"))
}
func (m *ObjectMeta) GetManagedFields() []metav1.ManagedFieldsEntry {
	panic(fmt.Errorf("GetManagedFields is not implemented"))
}
func (m *ObjectMeta) SetManagedFields(managedFields []metav1.ManagedFieldsEntry) {
	panic(fmt.Errorf("SetManagedFields is not implemented"))
}
