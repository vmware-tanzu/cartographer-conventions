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

package fake

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/util/webhook"

	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

var (
	caCert = []byte(`-----BEGIN CERTIFICATE-----
MIICwjCCAaoCCQDa3gTI7HXMXjANBgkqhkiG9w0BAQsFADAiMSAwHgYDVQQDDBdt
YWluX29waW5pb25fd2ViaG9va19jYTAgFw0yMTAxMDYwNjQ5MjZaGA8yMjk0MTAy
MjA2NDkyNlowIjEgMB4GA1UEAwwXbWFpbl9vcGluaW9uX3dlYmhvb2tfY2EwggEi
MA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDXzfU46m2RmYC5MBD7sdaulnSI
AZ3ZirbZuRm6tHzUVpG0Ii3hYaodxWGV/QgtopLMS2/b8hStlalkZjE+NfUITRI2
kSpnrjaXv2AeG3LppIhFnMz62yhIFzt/YCtbJQEpHM7FMVZU2bvXPATCz8C8jvgw
wVauPwhWXRAaEen1FQMYar3Ppi70jxh/GgAtdPkZQ0iJIc1sQPi4NZPvfDQuge1x
WA8yrDHnTZQ4JF3YCyyUrH+37qt1tXBawR7AINVpIuCGNb99pl7O81kNmB20yFDS
7Ii+iStgUqLyEnBC3qdEwZilafM/RfPxmaP+CEFjWJUeFIF4H1gPoFkfAq+XAgMB
AAEwDQYJKoZIhvcNAQELBQADggEBAENV3iPX7wXu5OXRlLono40Q8m0WhCxrkKz+
OewvCs0du4KTTUxOHNnOZY6NrIsZT/RpsNIkdA3ysD1mNNwongEW2kX9K9pgZR0D
ck59/4I5jb/URkhmkbLCyPMy5H6Lo3OVwMhu+nZX0tUuosYuRqLoxq88ZyoQQeyn
BODQi5yRy1AdSF+mjoTAml8Cso/0XxWrWmvey5vj485MK1kekmBA6yjt6dXKeff9
21hDMQYpx50rxWDKswoDNbO/N0jZhPG6ob7rmYRkfUvLLbpMlwi2yQyizJmUv3gJ
VwxFwjL0XutcwQKCwB99NCl31j0xeG3WiRHicd3u5+XPQEoTye4=
-----END CERTIFICATE-----`)
	serverKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAmrnj3GzFx+r0sCHZYmN5i8ivKme/dlon5qfNn6CkQQqyVk4B
0FweRTpx4eqpmBdjHDeYrxq+kzVQAKrv0vFe1lwdY0KV68nvlHQUiH7qgQwiI0lo
5kyrCX8/52KIYy+JLqar6xSf0/5TXf0i6MxDbXllPYupjZtf4gG6TFdyaRzsnkcb
6nXDQpUPeyCXabmtE4Az4zNeJ3jwbVbgz+JjYXZH/+Ka+ajOG88mI4M1d7mEuXXi
pOEfbeATyQF4nWwvclG3cUwPF3yCn2YKKkqX83Z8quRDjMYzUQ5kfzFpz/d+sHUQ
gpjRGgBbPUtpLm3aDwkLrR8bsITnQPuk+lSuYwIDAQABAoIBAQCH1ZA8UGXmF9hO
1LiijtADLuDQ6poEzitfbIuxivcIftqHuB4RjP2qKyAVhMz0z/tbp1dsyp7qX5Bn
tamr1+k5aU8HeEpj4Tlqa+om1r0LI9rIfccQ/9fcE5HHkhJIeVAG253sWIPkOc94
oSXHmKPNdRizSmxE/FXV9UxXfyHbMoLxHC8YOhMX9OGUmDJBDICTwsn2bxlNFMaK
lSjUExkPhQJ6leaqEYznRjYvyaGt+xJUhHBUXeWmX+VpB3bOEy4ncOV3HC1mt82q
JKiNGRDuBnA7oQjG5VCFGLQijKxUcdjjWJz6Y+O/7ktFERLQ1V1+gBMh/m+MWZ+1
oJNz4taJAoGBAMnF0KC5fnFFKfWclKNwOWm2l1e6kNbB3rVHBslCVVt5+xnis+oB
ffWMDOCIxaLIW7w+1qqMGXgHeSlfYhdHEqxv6KO9H4KLb7r5NGwR4DMe2QkhuvbH
XAt7qHQwYdFijHWclsVOz2xgzxtvfw/zcFYFGDIvSWyj2pS4Fp+9ZKd3AoGBAMRP
N+8ln6fNWvgnUrmENbI6pagQOrKFMqGh8nE40xLh/Ftl47CixbNox/KGJ1WvQHNr
no+wmV7Yh0Olnqa4SXK7AnkDfwkOkvcUqwyd4is9ncir61tpAyBaKjC0Lxcjhmla
sngzIPZ3aUoTe/cMhzZA/dSOiRRR/LsOV3pogEN1AoGAIOvSt3ash8S2LOnoYqZb
58Cv/tNk8HVfZgp5s/rLvIoxiy6vFj46FAdOzo/iV0YDmbpTAi6rtSbbAQIcGhox
lMsJlTW1X3Jqv4ILqJpeD1k4JkJHpB4xCXqaqKKAQ06mBkaPXxAVzeQZxqsxeyPI
L3DTWtTWURCHCH7kyhl3w88CgYEAgbmH0PUf6BeAQfRaalW/1iODTOhMoaP7rWwD
dmaCtTu5M/zE1fj6hHB9kPquC6VgBeXcRkABWffkiwNrL+kgQDzsiWOSEz4aSETU
M+YxizmQhwd05FckxcBPmRe49qV3MS/KODwxUC3g2h6+EKeqwmN4WXpHg7IaPNJh
ZHaiK/ECgYBcjFtFhda1FiWFsF8DJQ0M0q8Z6/E7A26cDtcKZNUOgA7I+A757gpM
QRErOF+eQnWJxZePRdxseq3rHHJQRGoC1BL5oahoJmvRFpxjrmsSaGFjqTXGmIIk
PT1WLVncmtBr5hygUwWv5/NGhG5miSsHTCbUzKYi9pz4WEUOtm13Fg==
-----END RSA PRIVATE KEY-----`)

	serverCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDLjCCAhagAwIBAgIJAOJVhlLdlioUMA0GCSqGSIb3DQEBBQUAMCIxIDAeBgNV
BAMMF21haW5fb3Bpbmlvbl93ZWJob29rX2NhMCAXDTIxMDEwNjA2NDkyNloYDzIy
OTQxMDIyMDY0OTI2WjAjMSEwHwYDVQQDDBh3ZWJob29rLXRlc3QuZGVmYXVsdC5z
dmMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCauePcbMXH6vSwIdli
Y3mLyK8qZ792Wifmp82foKRBCrJWTgHQXB5FOnHh6qmYF2McN5ivGr6TNVAAqu/S
8V7WXB1jQpXrye+UdBSIfuqBDCIjSWjmTKsJfz/nYohjL4kupqvrFJ/T/lNd/SLo
zENteWU9i6mNm1/iAbpMV3JpHOyeRxvqdcNClQ97IJdpua0TgDPjM14nePBtVuDP
4mNhdkf/4pr5qM4bzyYjgzV3uYS5deKk4R9t4BPJAXidbC9yUbdxTA8XfIKfZgoq
Spfzdnyq5EOMxjNRDmR/MWnP936wdRCCmNEaAFs9S2kubdoPCQutHxuwhOdA+6T6
VK5jAgMBAAGjZDBiMAkGA1UdEwQCMAAwCwYDVR0PBAQDAgXgMB0GA1UdJQQWMBQG
CCsGAQUFBwMCBggrBgEFBQcDATApBgNVHREEIjAghwR/AAABghh3ZWJob29rLXRl
c3QuZGVmYXVsdC5zdmMwDQYJKoZIhvcNAQEFBQADggEBAGLYBdYp5Y9DyDLdjlH/
u6aBnKyn2RdgfYh6IcztGAt4HfP8Yq9kyTjzaprya0NVkyZM4vZFL7jbBbAJpgkZ
9b3/WpxBlTvDcezZAkShBG0R6C5HH6m7XybqCMOsu83wP5FuJdBlEHynALPefgbj
Nljvv6JPzYPSJmdvSmKvLHipGBbJlHFix0BL1B3wiyS16l41Y3SizNMSiXtnricY
ixeuIy++RQ5EUI3vgDohZu8ETvrhZF6AXMruQUnXu5PrqJC+drFsY9B+ybIyvKbt
kE8uPov/rJMR4N7NvlibdOeVfj9zC7CPb+PMrypGFU7sCRcqzNlAVBNNCBErlSb8
4mU=
-----END CERTIFICATE-----`)
)

// NewFakeConventionServer returns a webhook test HTTPS server with fixed webhook test certs.
func NewFakeConventionServer() (*httptest.Server, []byte, error) {
	// Create the test webhook server
	cert, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, nil, err
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(webhookHandler))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	return server, caCert, nil
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	reqObj := &webhookv1alpha1.PodConventionContext{}
	if r.Body != nil {
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		decoder := json.NewDecoder(bytes.NewBuffer(reqBody))
		if derr := decoder.Decode(reqObj); derr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	reqMetadata := reqObj.ObjectMeta.DeepCopy()
	validResponse := &webhookv1alpha1.PodConventionContext{
		ObjectMeta: *reqMetadata,
		Status: webhookv1alpha1.PodConventionContextStatus{
			Template: reqObj.Spec.Template,
		},
	}

	deafultContainerName := "test-workload"
	defaultImageName := "ubuntu"
	defaultEnvVar := corev1.EnvVar{
		Name:  "KEY",
		Value: "VALUE",
	}
	labelKey := "test-convention"
	defaultLabel := labelKey + "/default-label"

	urlParts := strings.SplitN(r.URL.Path, ";", 2)
	var path, registryHost string
	if len(urlParts) == 1 {
		path = r.URL.Path
	} else if len(urlParts) == 2 {
		path = urlParts[0]
		val, _ := url.ParseQuery(urlParts[1])
		registryHost = val.Get("host")
	}

	switch path {
	case "/svcpath":
		w.Header().Set("Content-Type", "application/json")
		validResponse.Status.Template.Spec.Containers = []corev1.Container{{
			Name:  deafultContainerName,
			Image: defaultImageName,
			Env:   []corev1.EnvVar{defaultEnvVar},
		}}
		validResponse.Status.AppliedConventions = []string{defaultLabel, "path/svcpath"}
		json.NewEncoder(w).Encode(validResponse)
	case "/addlogsidecar":
		w.Header().Set("Content-Type", "application/json")
		validResponse.Status.Template.Spec.Containers = append(validResponse.Status.Template.Spec.Containers, corev1.Container{
			Name:    "logsidecar",
			Image:   "prom/prometheus",
			Command: []string{"log-away"},
		})
		validResponse.Status.AppliedConventions = []string{defaultLabel, "path/addlogsidecar"}
		json.NewEncoder(w).Encode(validResponse)
	case "/hellosidecar":
		w.Header().Set("Content-Type", "application/json")
		validResponse.Status.Template.Spec.Containers = append(validResponse.Status.Template.Spec.Containers, corev1.Container{
			Name:    "hellosidecar",
			Image:   fmt.Sprintf("%s/hello", registryHost),
			Command: []string{"/bin/sleep", "100"},
		})
		validResponse.Status.AppliedConventions = []string{"path/hellosidecar"}
		json.NewEncoder(w).Encode(validResponse)
	case "/wrongobj":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode("invalid-resp-string")
	case "/badimage":
		w.Header().Set("Content-Type", "application/json")
		validResponse.Status.Template.Spec.Containers = append(validResponse.Status.Template.Spec.Containers, corev1.Container{
			Name:  "badimage",
			Image: fmt.Sprintf("%s/badimage", registryHost),
		})
		validResponse.Status.AppliedConventions = []string{"path/addbadimagesidecar"}
		json.NewEncoder(w).Encode(validResponse)
	case "/labelonly":
		w.Header().Set("Content-Type", "application/json")
		validResponse.Status.AppliedConventions = []string{defaultLabel, "path/addonlylabel"}
		json.NewEncoder(w).Encode(validResponse)
	case "/":
		w.Header().Set("Content-Type", "application/json")
		validResponse.Status.Template.Spec.Containers = append(validResponse.Status.Template.Spec.Containers, corev1.Container{
			Name:  deafultContainerName,
			Image: defaultImageName,
			Env:   []corev1.EnvVar{defaultEnvVar},
		})
		validResponse.Status.AppliedConventions = []string{defaultLabel}
		json.NewEncoder(w).Encode(validResponse)
	}
}

type serviceResolver struct {
	base url.URL
}

// NewStubServiceResolver returns a static service resolve that return the given URL or
// an error for the failResolve namespace.
func NewStubServiceResolver(base url.URL) webhook.ServiceResolver {
	return &serviceResolver{base}
}

func (f serviceResolver) ResolveEndpoint(namespace, name string, port int32) (*url.URL, error) {
	u := f.base
	return &u, nil
}
