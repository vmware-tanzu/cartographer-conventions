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
	"regexp"

	"github.com/CycloneDX/cyclonedx-go"
	// wokeignore:rule=master
	"github.com/Masterminds/semver"
	"k8s.io/apimachinery/pkg/util/sets"

	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

func NewDependenciesBOM(boms []webhookv1alpha1.BOM) DependenciesBOM {
	d := DependenciesBOM{}
	for _, b := range boms {
		// ignore errors, other boms may be in a different structure or not json
		if cdx, _ := b.AsCycloneDX(); cdx != nil {
			d.boms = append(d.boms, *cdx)
		}
	}
	return d
}

type DependenciesBOM struct {
	boms []cyclonedx.BOM
}

func (m *DependenciesBOM) Dependency(name string) *cyclonedx.Component {
	for _, b := range m.boms {
		if b.Components == nil {
			continue
		}
		for _, c := range *b.Components {
			if c.Name == name {
				return &c
			}
		}
	}
	return nil
}

func (m *DependenciesBOM) HasDependency(names ...string) bool {
	n := sets.NewString(names...)
	for _, b := range m.boms {
		if b.Components == nil {
			continue
		}
		for _, c := range *b.Components {
			if n.Has(c.Name) {
				return true
			}
		}
	}
	return false
}

func (m *DependenciesBOM) HasDependencyConstraint(name, constraint string) bool {
	d := m.Dependency(name)
	if d == nil {
		return false
	}
	v, err := semver.NewVersion(m.normalizeVersion(d.Version))
	if err != nil {
		return false
	}
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false
	}
	return c.Check(v)
}

func (m *DependenciesBOM) normalizeVersion(version string) string {
	r := regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+)\.`)
	return r.ReplaceAllString(version, "$1-")
}

type dependenciesBOMKey struct{}

func StashDependenciesBOM(ctx context.Context, props *DependenciesBOM) context.Context {
	return context.WithValue(ctx, dependenciesBOMKey{}, props)
}

func GetDependenciesBOM(ctx context.Context) *DependenciesBOM {
	value := ctx.Value(dependenciesBOMKey{})
	if deps, ok := value.(*DependenciesBOM); ok {
		return deps
	}

	return nil
}
