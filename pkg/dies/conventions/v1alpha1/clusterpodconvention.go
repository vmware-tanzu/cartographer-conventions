/*
Copyright 2022 VMware Inc.

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
	dieadmissionregistrationv1 "dies.dev/apis/admissionregistration/v1"
	diemetav1 "dies.dev/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
)

// +die:object=true
type _ = conventionsv1alpha1.ClusterPodConvention

// +die
type _ = conventionsv1alpha1.ClusterPodConventionSpec

func (d *ClusterPodConventionSpecDie) SelectorsDie(selectors ...*diemetav1.LabelSelectorDie) *ClusterPodConventionSpecDie {
	return d.DieStamp(func(r *conventionsv1alpha1.ClusterPodConventionSpec) {
		r.Selectors = make([]metav1.LabelSelector, len(selectors))
		for i := range selectors {
			r.Selectors[i] = selectors[i].DieRelease()
		}
	})
}

func (d *ClusterPodConventionSpecDie) WebookDie(fn func(d *ClusterPodConventionWebhookDie)) *ClusterPodConventionSpecDie {
	return d.DieStamp(func(r *conventionsv1alpha1.ClusterPodConventionSpec) {
		d := ClusterPodConventionWebhookBlank.
			DieImmutable(false).
			DieFeedPtr(r.Webhook)
		fn(d)
		r.Webhook = d.DieReleasePtr()
	})
}

// +die
type _ = conventionsv1alpha1.ClusterPodConventionWebhook

func (d *ClusterPodConventionWebhookDie) ClientConfigDie(fn func(d *dieadmissionregistrationv1.WebhookClientConfigDie)) *ClusterPodConventionWebhookDie {
	return d.DieStamp(func(r *conventionsv1alpha1.ClusterPodConventionWebhook) {
		d := dieadmissionregistrationv1.WebhookClientConfigBlank.
			DieImmutable(false).
			DieFeed(r.ClientConfig)
		fn(d)
		r.ClientConfig = d.DieRelease()
	})
}

func (d *ClusterPodConventionWebhookDie) CertificateDie(fn func(d *ClusterPodConventionWebhookCertificateDie)) *ClusterPodConventionWebhookDie {
	return d.DieStamp(func(r *conventionsv1alpha1.ClusterPodConventionWebhook) {
		d := ClusterPodConventionWebhookCertificateBlank.
			DieImmutable(false).
			DieFeedPtr(r.Certificate)
		fn(d)
		r.Certificate = d.DieReleasePtr()
	})
}

// +die
type _ = conventionsv1alpha1.ClusterPodConventionWebhookCertificate
