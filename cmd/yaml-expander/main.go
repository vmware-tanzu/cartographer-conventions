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

package main

import (
	"io"
	"log"
	"os"

	"k8s.io/apimachinery/pkg/util/yaml"
	sigyaml "sigs.k8s.io/yaml"
)

func main() {
	d := yaml.NewYAMLOrJSONDecoder(os.Stdin, 4096)
	for {
		doc := map[string]interface{}{}
		if err := d.Decode(&doc); err != nil {
			if err == io.EOF {
				return
			}
			log.Fatal(err)
		}
		if len(doc) == 0 {
			// skip empty documents
			continue
		}
		b, err := sigyaml.Marshal(doc)
		if err != nil {
			log.Fatal(err)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n---\n"))
	}
}
