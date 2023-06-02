/*
Copyright 2020-2023 VMware Inc.

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

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/util/webhook"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"

	conventionsv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/pkg/apis/conventions/v1alpha1"
	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

type Convention struct {
	Name           string
	SelectorTarget conventionsv1alpha1.SelectorTargetSource
	Selectors      []metav1.LabelSelector
	Priority       conventionsv1alpha1.PriorityLevel
	ClientConfig   admissionregistrationv1.WebhookClientConfig
}

func (o *Convention) Apply(ctx context.Context, conventionRequest *webhookv1alpha1.PodConventionContext, wc WebhookConfig) (*webhookv1alpha1.PodConventionContext, error) {
	cc := o.WebhookClientConfig()
	cm, err := NewClientManager(wc, webhookv1alpha1.GroupVersion, webhookv1alpha1.AddToScheme)
	if err != nil {
		return nil, err
	}
	webClient, err := cm.HookClient(cc)
	if err != nil {
		return nil, err
	}

	r := webClient.Post().Body(conventionRequest)
	enrichedIntent := &webhookv1alpha1.PodConventionContext{}
	res := r.Do(ctx)
	if res.Error() != nil {
		return nil, res.Error()
	}
	if err := res.Into(enrichedIntent); err != nil {
		return nil, err
	}
	return enrichedIntent, nil
}

func (o *Convention) WebhookClientConfig() webhook.ClientConfig {
	cc := webhook.ClientConfig{
		Name:     o.Name,
		CABundle: o.ClientConfig.CABundle,
	}
	if o.ClientConfig.URL != nil {
		cc.URL = *o.ClientConfig.URL
	}
	if o.ClientConfig.Service != nil {
		cc.Service = &webhookutil.ClientConfigService{
			Name:      o.ClientConfig.Service.Name,
			Namespace: o.ClientConfig.Service.Namespace,
		}
		if o.ClientConfig.Service.Path != nil {
			cc.Service.Path = *o.ClientConfig.Service.Path
		}
		if o.ClientConfig.Service.Port != nil {
			cc.Service.Port = *o.ClientConfig.Service.Port
		}
	}
	return cc
}
