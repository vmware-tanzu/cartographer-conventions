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

// Package v1 contains API Schema definition for the cert-manager.io/v1 API group

// This API group is a forked subset of https://github.com/jetstack/cert-manager/tree/v1.4.1/pkg/apis/certmanager/v1
// It is indended to enable interaction with the Cert Manager API without including unnecessary dependencies.

// Types have been lightly edited to avoid dependencies and support kubebuilder.

// +kubebuilder:object:generate=true
package v1
