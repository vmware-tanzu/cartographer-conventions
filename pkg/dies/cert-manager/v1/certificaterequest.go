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
	diecorev1 "dies.dev/apis/core/v1"
	diemetav1 "dies.dev/apis/meta/v1"

	certmanagerv1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/thirdparty/cert-manager/v1"
)

// +die:object=true
type _ = certmanagerv1.CertificateRequest

// +die
type _ = certmanagerv1.CertificateRequestSpec

func (d *CertificateRequestSpecDie) IssuerRefDie(fn func(d *diecorev1.ObjectReferenceDie)) *CertificateRequestSpecDie {
	return d.DieStamp(func(r *certmanagerv1.CertificateRequestSpec) {
		d := diecorev1.ObjectReferenceBlank.
			DieImmutable(false).
			DieFeed(r.IssuerRef)
		fn(d)
		r.IssuerRef = d.DieRelease()
	})
}

func (d *CertificateRequestSpecDie) AddExtra(key string, values ...string) *CertificateRequestSpecDie {
	return d.DieStamp(func(r *certmanagerv1.CertificateRequestSpec) {
		if r.Extra == nil {
			r.Extra = make(map[string][]string)
		}
		r.Extra[key] = values
	})
}

// +die
type _ = certmanagerv1.CertificateRequestStatus

func (d *CertificateRequestStatusDie) ConditionsDie(conditions ...*diemetav1.ConditionDie) *CertificateRequestStatusDie {
	return d.DieStamp(func(r *certmanagerv1.CertificateRequestStatus) {
		r.Conditions = make([]certmanagerv1.CertificateRequestCondition, len(conditions))
		for i := range conditions {
			c := conditions[i].DieRelease()
			// coerce metav1.Condition to certmanagerv1.CertificateRequestCondition
			r.Conditions[i] = certmanagerv1.CertificateRequestCondition{
				Type:               certmanagerv1.CertificateRequestConditionType(c.Type),
				Status:             c.Status,
				Reason:             c.Reason,
				Message:            c.Message,
				LastTransitionTime: &c.LastTransitionTime,
			}
		}
	})
}

var (
	CertificateRequestConditionReadyBlank          = diemetav1.ConditionBlank.Type(string(certmanagerv1.CertificateRequestConditionReady))
	CertificateRequestConditionInvalidRequestBlank = diemetav1.ConditionBlank.Type(string(certmanagerv1.CertificateRequestConditionInvalidRequest))
	CertificateRequestConditionApprovedBlank       = diemetav1.ConditionBlank.Type(string(certmanagerv1.CertificateRequestConditionApproved))
	CertificateRequestConditionDeniedBlank         = diemetav1.ConditionBlank.Type(string(certmanagerv1.CertificateRequestConditionDenied))
)
