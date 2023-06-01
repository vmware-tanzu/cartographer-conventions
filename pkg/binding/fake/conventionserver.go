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

package fake

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/util/webhook"

	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

var (
	caCert = []byte(`
-----BEGIN CERTIFICATE-----
MIIDcDCCAligAwIBAgIUFLBZgUpI8BBzKlo508O/3/SJ49YwDQYJKoZIhvcNAQEL
BQAwOjETMBEGA1UEChMKVk13YXJlIEluYzEOMAwGA1UECxMFVGFuenUxEzARBgNV
BAMTCkNJIFJvb3QgQ0EwHhcNMjIwMzIxMTMwNTAwWhcNMzIwMzE4MTMwNTAwWjBC
MRMwEQYDVQQKEwpWTXdhcmUgSW5jMQ4wDAYDVQQLEwVUYW56dTEbMBkGA1UEAxMS
Q0kgSW50ZXJtZWRpYXRlIENBMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKC
AQEAwW9IZ2c+mZGwV6sOrsHsMWToXUbzlEiKmr7Ca7lkgK63Lgr4FUv1vuec7P9c
b+s52jipbvtB94eeGSGn0+vu2601XN9wGO6vtoXnGTvxiss71sPjMKDsMqlaviRS
g0en+usFrVKfc96kiRjzeeiKI1OGB6T/0u+x6+adhvwY0FH7o4Wwxa9LlyF1ryro
vyfmpNC/73ahVQpTnKRY8HIyRXUEiW05ZZXp48Zd/MVS964W6UxPHTZjjGCSOHkh
plqoezLpmvZsUJrigFoKjrf6xbw7umzW5UhUrizkUKGIxN47HLfqXqu7ZFKvVAMO
kkjqQLeT6EGKcaTorYBRktnDzQIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAQYwEgYD
VR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQUjMgo+KVAPQ30oyoUbWmPQw2lneIw
HwYDVR0jBBgwFoAUgY1snKL7IhaKhWi0z0/oAvmys+swDQYJKoZIhvcNAQELBQAD
ggEBAEwOlBlH98kOjIzbbEifzC08txqSnG89YafGzcVq0oqlGnHPQSjLcqGQ6gn9
r2/7/++mFuhpPO1ugJUlPFBF7xydvnwr+08PViCcrdrASfAvnc3ZoRXesN8uOLzv
S+/+KC2gpFwHM+5KBwuE9o78Fg4p7KTN2gh9cNUQ5Y398puyNMiUdiUP9VjBMpkf
6pzQjGXncBcTBrC7QMgyTs1RewOMI0RvcrYHPohrfm9pU/ie+bOwsKcuKM213CNp
D9UvcEKh5ejmO4gIfdi1h7+0rAzUI3feThkfDVHsH10Dad7RjgwETqI/FIfgcmC/
TJhonscDvman25+HoHU40frJ7ds=
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIDRDCCAiygAwIBAgIUQKqJe1+SHG71bCAWviZuuuku/N0wDQYJKoZIhvcNAQEL
BQAwOjETMBEGA1UEChMKVk13YXJlIEluYzEOMAwGA1UECxMFVGFuenUxEzARBgNV
BAMTCkNJIFJvb3QgQ0EwHhcNMjIwMzIxMTMwMzAwWhcNMjcwMzIwMTMwMzAwWjA6
MRMwEQYDVQQKEwpWTXdhcmUgSW5jMQ4wDAYDVQQLEwVUYW56dTETMBEGA1UEAxMK
Q0kgUm9vdCBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMwyocsO
p7ZNaqBPLBmlYbMk/PlGwgteYTDuZ9PbESVN/kw7ZtU+Y9bWl1iJ9Dy+dApaEQq1
cjhLeCYDVdRUwtc/oKieWWbxnsmVwqJs7TPDT2i7nlucMSRKsRs/uP82EVmgDdPb
LhpSy5oA4af6uNuQI0MsLjZoe22BWd40X3G32Z3qeLw3A15NhljOjoxXdmbf/+vr
NzFfLJj+D0sm3drV2AQ5q8pWqIT81p0RdA54MGYA5NcZ2lfbaWKt3PcA+GEVqKLq
5JPSWuEb0jxvLjTpp7SjsXBkWbVSbHpYF1833XfvdcSWU/Fooiqq3Wqlbgju24oI
uqmmM/1kdGVP5I0CAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1UdEwEB/wQF
MAMBAf8wHQYDVR0OBBYEFIGNbJyi+yIWioVotM9P6AL5srPrMA0GCSqGSIb3DQEB
CwUAA4IBAQA0Vj01pRD1A0fOFrLNEQijvrKW0BTQdcX6wWgM1f0HmhksUNwqluGW
wtQWJXUP1S51EAimsU5ML2azqKeGV1GWD+3cLEKG8pj++1o3jZ9IDqisIDK/AaWr
+7KKGA3fuW8uWVEILp2U8XzU02VKSs0HNYUEFafNGpXOLG/7f6UpoX72oIHed5if
WwqBatee/2vB40VjGR3Q9lr3kttlTUuzFWTqHA1g0w7DyfBCRMGFI4G6+VzCLwCU
JJnM7ND1pTr+Eoehsnw+9VcWHeU3f6a8SCxXJyZCREs/IkNRrUdpEJPUB/k0IGpf
PyqFBQvmAWtQQm985XgIv8ka0AG7BaQx
-----END CERTIFICATE-----
`)
	serverKey = []byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAnA35Cz5jzLb40jayeer1GHLT/TJ7se6ipY1ym6usyUYhTd10
RTnxs10nI6ht4eZXtVkaAYn5WRyODZxWoN5ZzOx69OgtiXJ+W8cmwqHojYQeQLSE
VsOEOEcwHvOMEnwpBZJYZqcdT3qLSzGyCEjMFrTIC0l5lkfCXFGyEVYa9QzCRns6
izFiEnx2ri9fO9fLbIee2Y8iQbQpqGzgj2u0i0W0P9MUkW0S7hR6SPjXjNWK5qmw
NTkm8Sqgcv8ntEcrHi/8hfThQP1ajgjSi8dThoADAdGZa+Kr64Z/4ORpsO/Kb5U9
vgIDFnoR7ZJAfbYZgB9/JHW/Lm/XyWzbAdoduwIDAQABAoIBAQCOdA7gXa6apHhU
5Mtdkcb073Vmj5vs3Esq1wlE450SCuvB+aL2wqNJuYJOAaV07mEoUVL1Dq9I0lE2
SX2m0fKlp0XCpONUseh4/T37s/LmpDE9ncukrEvZV9qslmRKR37m8CW0Z17RO6tG
E/JRr6pmG9b0vri4H39j6MulGbX39KWxIPMSRHdnu1BmXIIRKczk6oTor1TtQ7N3
zELgFs/0ZKkPPU3nObpMrywK35bqTeGFjoMJA8SdiyjrQHdA1x+x44tXHJgPjAzB
7bvbIuiUNmIhBh4nFmp0g46hcRzj9XcHfqYAT+EuvNSLW0u5zyQncgX5awJh7S8v
ILSsvYTBAoGBAMO4dvf2iYRJ5QhHeVMI0X2JkUOZtziwV+5QHEZyGQyiZKh3eBbN
4pZ8Y5qKKj6/+6oxWmC6E9HVZyHRqrF58gQvMLhAiFIhB1ApS9Gcl+kX2RgSzbMh
UO7qsmmSZGeWa9XZtW/hFAwMqs7UePZK/a5NpEKW79xR2OsIUbidK5nhAoGBAMwe
DeVXZtZo5NT4MI/lUN0mPMDh7qUWCFYDyA0xMTodS8djsyQergj3G8J9Fc2D9gFe
fXMyY2lgn+lb4uMGHNGwShulfKb3uDJySLKE/Zlg1J4XXLz6wah8KpFuOPKAYiD5
JK3fTPs5lyySYiM/tmLkpJe7I4bLzM6wDL6SW0MbAoGAdIw6O/qRdTdTrZRySOHt
beYnnKvCkX1hP0ZxL/nttLpXWoKZ/mpnzdkQrwwrj+ZfBMAS45qrBr8fhOIH1Vua
pKc9SdsT0mRcqH2O6qlnRKSw4EcCOvNR8JPN3lQQeib23AeipZbQi0RXyoZ36aJK
YitV71lWSEps87imgVsGhcECgYBMtTq5pogCKbddhcwSN7aU9Yq9XermZYpKcO9c
bdE3Ks1QqFopR9JVki//fiyUaHQp/Y2dniEX9/UAqMRyVti7wMmI7D8VLGEvrB0/
4ZTAcFBW/Saf6oievdLthoOmNrMp+xdatGFkxDbYzEZPQuFS9uQYFX77aFmWjziq
4aukYwKBgCxoQK3ozB4e808kq0D22U+QWbRRsfhuNfDOtBJx+l6pOMvryEu0XRzi
KkLMg002hBQVRVb8hKcGmYEpkI+FqJH5BLpZd3omjObL+02adXh5EdYJjHSiL2VP
2Y5g1ABFtwcySOGKNRa9xhmVdhbdwHK1FRz60CyaRsET6MIBY9oj
-----END RSA PRIVATE KEY-----
`)

	serverCert = []byte(`
-----BEGIN CERTIFICATE-----
MIIDyjCCArKgAwIBAgIUZoTrkkhNxkPEesRXLEFydmz7FZswDQYJKoZIhvcNAQEL
BQAwQjETMBEGA1UEChMKVk13YXJlIEluYzEOMAwGA1UECxMFVGFuenUxGzAZBgNV
BAMTEkNJIEludGVybWVkaWF0ZSBDQTAeFw0yMjAzMjExMzA5MDBaFw0zMjAzMTgx
MzA5MDBaMCUxEzARBgNVBAoTClZNd2FyZSBJbmMxDjAMBgNVBAsTBVRhbnp1MIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnA35Cz5jzLb40jayeer1GHLT
/TJ7se6ipY1ym6usyUYhTd10RTnxs10nI6ht4eZXtVkaAYn5WRyODZxWoN5ZzOx6
9OgtiXJ+W8cmwqHojYQeQLSEVsOEOEcwHvOMEnwpBZJYZqcdT3qLSzGyCEjMFrTI
C0l5lkfCXFGyEVYa9QzCRns6izFiEnx2ri9fO9fLbIee2Y8iQbQpqGzgj2u0i0W0
P9MUkW0S7hR6SPjXjNWK5qmwNTkm8Sqgcv8ntEcrHi/8hfThQP1ajgjSi8dThoAD
AdGZa+Kr64Z/4ORpsO/Kb5U9vgIDFnoR7ZJAfbYZgB9/JHW/Lm/XyWzbAdoduwID
AQABo4HUMIHRMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAM
BgNVHRMBAf8EAjAAMB0GA1UdDgQWBBRqOq+sBqyhbzqAUdb+/wPdHi77kjAfBgNV
HSMEGDAWgBSMyCj4pUA9DfSjKhRtaY9DDaWd4jBcBgNVHREEVTBTgglsb2NhbGhv
c3SCGHdlYmhvb2stdGVzdC5kZWZhdWx0LnN2Y4Imd2ViaG9vay10ZXN0LmRlZmF1
bHQuc3ZjLmNsdXN0ZXIubG9jYWyHBH8AAAEwDQYJKoZIhvcNAQELBQADggEBAHnD
zbHbDn+HIV6XNv69gR4wXgGadDfLa6MGUI441l7dylyK7uAxyyTlNBhD8FyDwuiU
VODXO5QQYbjiSc0J/FyGSxBsACftgGt4YS9MnKa/7pV4bhVpwkrYG3SsqY2yfNI0
dWaXhMyDj6t2QwKUTHycv4HmWaq4cW0QduNS+UZ6bkdq0qQyU9cIHvwsuxE08ubs
8QNitqML0MeU8HDWQzdq91e1mY/YULocvS3GNmXHIyOLoZ2qk7Zxv6DfSEvU+0EC
fTx+WenX4hYIyLyh74uNlku5H0goG4QiOnlZkXi0o6G/fC8NywtybUYE4tt6v/p5
52o8ORleKRYvSUjCc88=
-----END CERTIFICATE-----
`)
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
		reqBody, err := io.ReadAll(r.Body)
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
