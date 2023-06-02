/*
Copyright 2022-2023 VMware Inc.

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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"

	"github.com/vmware-tanzu/cartographer-conventions/webhook"
	webhookv1alpha1 "github.com/vmware-tanzu/cartographer-conventions/webhook/api/v1alpha1"
)

func conventionHandler(w http.ResponseWriter, r *http.Request) {
	wc := &webhookv1alpha1.PodConventionContext{}
	if r.Body != nil {
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		decoder := json.NewDecoder(bytes.NewBuffer(reqBody))
		if derr := decoder.Decode(wc); derr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// dump request body as yaml
		d, err := yaml.Marshal(wc)
		if err == nil {
			fmt.Println("---")
			fmt.Println(string(d))
		}
	}
	w.Header().Set("Content-Type", "application/json")
	wc.Status.AppliedConventions = []string{"dumper"}
	wc.Status.Template = wc.Spec.Template
	if err := json.NewEncoder(w).Encode(wc); err != nil {
		return
	}
}

func main() {
	ctx := context.Background()
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	zapLog, err := zap.NewProductionConfig().Build()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	logger := zapr.NewLogger(zapLog)
	ctx = logr.NewContext(ctx, logger)

	http.HandleFunc("/", conventionHandler)
	log.Fatal(webhook.NewConventionServer(ctx, fmt.Sprintf(":%s", port)))
}
