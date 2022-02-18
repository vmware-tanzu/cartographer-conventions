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

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"

	"github.com/vmware-tanzu/cartographer-conventions/samples/spring-convention-server/resources"
	"github.com/vmware-tanzu/cartographer-conventions/webhook"
)

func addSpringBootConventions(template *corev1.PodTemplateSpec, images []webhook.ImageConfig) ([]string, error) {
	imageMap := make(map[string]webhook.ImageConfig)
	for _, config := range images {
		imageMap[config.Image] = config
	}

	var appliedConventions []string
	for i := range template.Spec.Containers {
		// TODO how to best handle multiple spring boot containers?
		container := &template.Spec.Containers[i]
		image, ok := imageMap[container.Image]
		if !ok {
			return nil, fmt.Errorf("missing image metadata for %q", container.Image)
		}
		dependencyMetadata := resources.NewDependenciesBOM(image.BOMs)
		applicationProperties := resources.SpringApplicationProperties{}
		applicationProperties.FromContainer(container)

		ctx := context.Background()
		ctx = resources.StashSpringApplicationProperties(ctx, applicationProperties)
		ctx = resources.StashDependenciesBOM(ctx, &dependencyMetadata)
		for _, o := range resources.SpringBootConventions {
			// need to continue refining what metadata is passed to conventions
			if !o.IsApplicable(ctx, imageMap) {
				continue
			}
			appliedConventions = append(appliedConventions, o.GetId())
			if err := o.ApplyConvention(ctx, template, i, imageMap); err != nil {
				return nil, err
			}
		}
		applicationProperties.ToContainer(container)
	}
	return appliedConventions, nil
}

func main() {
	ctx := context.Background()
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}
	http.HandleFunc("/", webhook.ConventionHandler(ctx, addSpringBootConventions))
	log.Fatal(webhook.NewConventionServer(ctx, fmt.Sprintf(":%s", port)))
}
