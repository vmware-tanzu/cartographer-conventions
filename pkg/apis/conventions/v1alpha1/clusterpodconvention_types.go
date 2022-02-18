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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PriorityLevel string

const (
	// WantInjectAnnotation is the annotation that specifies that a particular
	// object wants injection of CAs.  It takes the form of a reference to a certificate
	// as namespace/name.
	WantInjectAnnotation = "conventions.carto.run/inject-ca-from"
	// EarlyPriority defines Early priority level for ClusterPodConvention
	EarlyPriority PriorityLevel = "Early"
	// NormalPriority defines Normal priority level for ClusterPodConvention
	NormalPriority PriorityLevel = "Normal"
	// LatePriority defines Late priority level for ClusterPodConvention
	LatePriority PriorityLevel = "Late"
)

type ClusterPodConventionSpec struct {
	// Label selector for workloads.
	// It must match the workload's pod template's labels.
	Selectors []metav1.LabelSelector       `json:"selectors,omitempty"`
	Priority  PriorityLevel                `json:"priority,omitempty"`
	Webhook   *ClusterPodConventionWebhook `json:"webhook,omitempty"`
}

type ClusterPodConventionWebhook struct {
	// ClientConfig defines how to communicate with the convention.
	ClientConfig admissionregistrationv1.WebhookClientConfig `json:"clientConfig"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories="conventions",scope=Cluster
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type ClusterPodConvention struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterPodConventionSpec `json:"spec"`
}

// +kubebuilder:object:root=true

type ClusterPodConventionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterPodConvention `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterPodConvention{}, &ClusterPodConventionList{})
}
