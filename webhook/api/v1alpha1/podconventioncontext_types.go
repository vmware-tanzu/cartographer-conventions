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
	"encoding/json"

	"github.com/CycloneDX/cyclonedx-go"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodConventionContextSpec struct {
	Template    corev1.PodTemplateSpec `json:"template"`
	ImageConfig []ImageConfig          `json:"imageConfig"`
}

type PodConventionContextStatus struct {
	Template           corev1.PodTemplateSpec `json:"template"`
	AppliedConventions []string               `json:"appliedConventions"`
}

type ImageConfig struct {
	Image  string            `json:"image"`
	BOMs   []BOM             `json:"boms,omitempty"`
	Config ggcrv1.ConfigFile `json:"config"`
}

type BOM struct {
	Name string `json:"name"`
	Raw  []byte `json:"raw"`
}

func (b *BOM) AsCycloneDX() (*cyclonedx.BOM, error) {
	bom := &cyclonedx.BOM{}
	// TODO sniff the content to prevent non-cyclonedx boms from unmarshaling into a mismatched struct
	if err := json.Unmarshal(b.Raw, bom); err != nil {
		return nil, err
	}
	return bom, nil
}

type PodConventionContext struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PodConventionContextSpec   `json:"spec"`
	Status            PodConventionContextStatus `json:"status"`
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new PodConventionContext.
func (in *PodConventionContext) DeepCopy() *PodConventionContext {
	// writing our own deep copy method using json marshaling to sidestep the cyclonedx BOM not
	// having its own DeepCopy method.

	if in == nil {
		return nil
	}
	copy, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	out := &PodConventionContext{}
	if err := json.Unmarshal(copy, out); err != nil {
		panic(err)
	}
	return out
}

// DeepCopyObject is a deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PodConventionContext) DeepCopyObject() runtime.Object {
	// NB: the webhook client expects the resource to implement runtime.Object, we don't expect
	// these methods to actually be used.
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func init() {
	SchemeBuilder.Register(&PodConventionContext{})
}
