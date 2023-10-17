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

package binding

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

type Conventions []Convention

func (c *Conventions) FilterAndSort(collectedLabels map[string]labels.Set) (Conventions, error) {
	filteredConventions, err := c.Filter(collectedLabels)
	if err != nil {
		return nil, err
	}
	return filteredConventions.Sort(), nil
}

func (c *Conventions) Filter(collectedLabels map[string]labels.Set) (Conventions, error) {
	originalOrder := *c

	var filteredSources Conventions
	for _, source := range originalOrder {
		selectors := source.Selectors
		if len(selectors) == 0 {
			selectors = []metav1.LabelSelector{
				// an empty selector matches everything
				{},
			}
		}
		for _, selector := range selectors {
			sourceLabels, err := metav1.LabelSelectorAsSelector(&selector)
			if err != nil {
				return nil, fmt.Errorf("unable to convert label selector for ClusterPodConvention %q: %v", source.Name, err)
			}

			if sourceLabels.Matches(collectedLabels[string(source.SelectorTarget)]) {
				filteredSources = append(filteredSources, source)
				break
			}
		}
	}

	return filteredSources, nil
}

func (c *Conventions) Sort() Conventions {
	originalConventions := *c
	sort.Slice(originalConventions, func(i, j int) bool {
		a := originalConventions[i]
		b := originalConventions[j]
		if a.Priority == b.Priority {
			return a.Name < b.Name
		}
		return a.Priority == conventionsv1alpha1.EarlyPriority || b.Priority == conventionsv1alpha1.LatePriority
	})
	return originalConventions
}

func (c *Conventions) Apply(ctx context.Context,
	parent *conventionsv1alpha1.PodIntent,
	wc WebhookConfig,
	rc RegistryConfig,
) (*corev1.PodTemplateSpec, error) {
	log := logr.FromContextOrDiscard(ctx)
	if parent == nil {
		return nil, fmt.Errorf("PodIntent value cannot be nil")
	}
	workload := parent.Spec.Template.AsPodTemplateSpec()
	appliedConventions := []string{}
	if str := workload.Annotations[conventionsv1alpha1.AppliedConventionsAnnotationKey]; str != "" {
		appliedConventions = strings.Split(str, "\n")
	}
	for _, convention := range *c {
		// fetch metadata for workload
		imageConfigList, err := rc.ResolveImageMetadata(ctx, workload)
		if err != nil {
			log.Error(err, "fetching metadata for Images failed")
			return nil, fmt.Errorf("failed to fetch metadata for Images: %v", err)
		}
		conventionRequestObj := &webhookv1alpha1.PodConventionContext{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", parent.GetName(), convention.Name),
			},
			Spec: webhookv1alpha1.PodConventionContextSpec{
				ImageConfig: imageConfigList,
				Template:    *workload,
			},
		}
		conventionResp, err := convention.Apply(ctx, conventionRequestObj, wc)
		if err != nil {
			log.Error(err, "failed to apply convention", "Convention", convention)
			return nil, fmt.Errorf("failed to apply convention with name %s: %s", convention.Name, err.Error())
		}
		workloadDiff := cmp.Diff(workload, conventionResp.Status.Template, cmpopts.EquateEmpty())
		log.Info("applied convention", "diff", workloadDiff, "convention", convention.Name)

		workload = &conventionResp.Status.Template // update pod spec before calling another webhook

		for _, appliedConvention := range conventionResp.Status.AppliedConventions {
			labelWithPrefix := fmt.Sprintf("%s/%s", convention.Name, appliedConvention)
			// append to the original list so that an convention cannot remove the history
			appliedConventions = append(appliedConventions, labelWithPrefix)
		}
		if workload.Annotations == nil {
			workload.Annotations = map[string]string{}
		}
		workload.Annotations[conventionsv1alpha1.AppliedConventionsAnnotationKey] = strings.Join(appliedConventions, "\n")
	}
	return workload, nil
}
