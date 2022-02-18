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

package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

type ImageMetadata = map[string]webhookv1alpha1.ImageConfig

type Convention interface {
	GetId() string
	IsApplicable(ctx context.Context, metadata ImageMetadata) bool
	ApplyConvention(ctx context.Context, target *corev1.PodTemplateSpec, containerIdx int, metadata ImageMetadata) error
}

var _ Convention = (*BasicConvention)(nil)

type BasicConvention struct {
	Id         string
	Applicable func(ctx context.Context, metadata ImageMetadata) bool
	Apply      func(ctx context.Context, target *corev1.PodTemplateSpec, containerIdx int, metadata ImageMetadata) error
}

func (o *BasicConvention) GetId() string {
	return o.Id
}

func (o *BasicConvention) IsApplicable(ctx context.Context, metadata ImageMetadata) bool {
	if o.Applicable == nil {
		return true
	}
	return o.Applicable(ctx, metadata)
}

func (o *BasicConvention) ApplyConvention(ctx context.Context, target *corev1.PodTemplateSpec, containerIdx int, metadata ImageMetadata) error {
	return o.Apply(ctx, target, containerIdx, metadata)
}

// setAnnotation sets the annotation on PodTemplateSpec
func setAnnotation(pts *corev1.PodTemplateSpec, key, value string) {
	if pts.Annotations == nil {
		pts.Annotations = map[string]string{}
	}
	pts.Annotations[key] = value
}

// setLabel sets the label on PodTemplateSpec
func setLabel(pts *corev1.PodTemplateSpec, key, value string) {
	if pts.Labels == nil {
		pts.Labels = map[string]string{}
	}
	pts.Labels[key] = value
}

func findEnvVar(container corev1.Container, name string) *corev1.EnvVar {
	for i := range container.Env {
		e := &container.Env[i]
		if e.Name == name {
			return e
		}
	}
	return nil
}

func findContainerPort(ps corev1.PodSpec, port int32) (string, *corev1.ContainerPort) {
	for _, c := range ps.Containers {
		for _, p := range c.Ports {
			if p.ContainerPort == port {
				return c.Name, &p
			}
		}
	}
	return "", nil
}
