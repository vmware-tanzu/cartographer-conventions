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
	"github.com/vmware-labs/reconciler-runtime/apis"
)

const (
	AppliedConventionsAnnotationKey = "conventions.carto.run/applied-conventions"
)

const (
	PodIntentConditionReady              = apis.ConditionReady
	PodIntentConditionConventionsApplied = "ConventionsApplied"
)

var podintentCondSet = apis.NewLivingConditionSetWithHappyReason(
	"ConventionsApplied",
	PodIntentConditionConventionsApplied,
)

func (s *PodIntent) GetConditionsAccessor() apis.ConditionsAccessor {
	return &s.Status
}

func (s *PodIntent) GetConditionSet() apis.ConditionSet {
	return podintentCondSet
}

func (s *PodIntentStatus) InitializeConditions() {
	conditionManager := podintentCondSet.Manage(s)
	conditionManager.InitializeConditions()
	// reset existing managed conditions
	conditionManager.MarkUnknown(PodIntentConditionConventionsApplied, "Initializing", "")
}
