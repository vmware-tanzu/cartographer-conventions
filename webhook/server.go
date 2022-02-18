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

package webhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

const (
	CertMountPath = "/config/certs"
)

type Convention func(*corev1.PodTemplateSpec, []webhookv1alpha1.ImageConfig) ([]string, error)
type ImageConfig = webhookv1alpha1.ImageConfig

func NewConventionServer(ctx context.Context, addr string) error {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	watcher := certWatcher{
		CrtFile: filepath.Join(CertMountPath, "tls.crt"),
		KeyFile: filepath.Join(CertMountPath, "tls.key"),
	}
	if err := watcher.Load(ctx); err != nil {
		return err
	}
	go watcher.Watch(ctx)

	server := &http.Server{
		Addr: addr,
		TLSConfig: &tls.Config{
			GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert := watcher.GetCertificate()
				return cert, nil
			},
			PreferServerCipherSuites: true,
			MinVersion:               tls.VersionTLS13,
		},
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	return server.ListenAndServeTLS("", "")
}

func ConventionHandler(ctx context.Context, convention Convention) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := loggerFromContext(ctx)
		logInfo(logger, "received request")
		wc := &webhookv1alpha1.PodConventionContext{}
		if r.Body != nil {
			reqBody, err := ioutil.ReadAll(r.Body)
			if err != nil {
				logError(logger, err, "error reading request body")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			decoder := json.NewDecoder(bytes.NewBuffer(reqBody))
			if derr := decoder.Decode(wc); derr != nil {
				logError(logger, derr, "error decoding request body into PodConventionContext type")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		pts := wc.Spec.Template.DeepCopy()
		appliedConventions, err := convention(pts, wc.Spec.ImageConfig)
		if err != nil {
			logError(logger, err, "error applying conventions")
			w.WriteHeader(http.StatusInternalServerError)
		}
		wc.Status.AppliedConventions = appliedConventions
		wc.Status.Template = *pts
		if err := json.NewEncoder(w).Encode(wc); err != nil {
			logError(logger, err, "error encoding response")
			return
		}
	}
}

type certWatcher struct {
	CrtFile string
	KeyFile string

	m       sync.Mutex
	keyPair *tls.Certificate
}

func (w *certWatcher) Watch(ctx context.Context) error {
	logger := loggerFromContext(ctx)
	// refresh the certs periodically even if we miss a fs event
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := w.Load(ctx); err != nil {
					logError(logger, err, "error loading TLS key pair")
				}
			}
		}
	}()

	<-ctx.Done()
	return nil
}

func (w *certWatcher) Load(ctx context.Context) error {
	logger := loggerFromContext(ctx)
	w.m.Lock()
	defer w.m.Unlock()

	crt, err := ioutil.ReadFile(w.CrtFile)
	if err != nil {
		return err
	}
	key, err := ioutil.ReadFile(w.KeyFile)
	if err != nil {
		return err
	}
	keyPair, err := tls.X509KeyPair(crt, key)
	if err != nil {
		return err
	}
	leaf := keyPair.Leaf
	if leaf == nil {
		leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
		if err != nil {
			return err
		}
	}
	w.keyPair = &keyPair
	logInfo(logger, fmt.Sprintf("loaded TLS key pair (valid until %q)", leaf.NotAfter))
	return nil
}

func (w *certWatcher) GetCertificate() *tls.Certificate {
	w.m.Lock()
	defer w.m.Unlock()

	return w.keyPair
}
