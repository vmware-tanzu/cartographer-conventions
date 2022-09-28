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

package resources

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var SpringBootConventions = []Convention{
	&BasicConvention{
		Id: "spring-boot",
		Applicable: func(ctx context.Context, metadata ImageMetadata) bool {
			deps := GetDependenciesBOM(ctx)
			return deps.HasDependency("spring-boot")
		},
		Apply: func(ctx context.Context, target *corev1.PodTemplateSpec, containerIdx int, metadata ImageMetadata) error {
			deps := GetDependenciesBOM(ctx)
			setLabel(target, "conventions.carto.run/framework", "spring-boot")
			if springBootDependency := deps.Dependency("spring-boot"); springBootDependency != nil {
				setAnnotation(target, "boot.spring.io/version", springBootDependency.Version)
			}
			return nil
		},
	},
	&BasicConvention{
		Id: "spring-boot-graceful-shutdown",
		Applicable: func(ctx context.Context, metadata ImageMetadata) bool {
			deps := GetDependenciesBOM(ctx)
			return deps.HasDependencyConstraint("spring-boot", ">= 2.3.0-0") && deps.HasDependency(
				"spring-boot-starter-tomcat",
				"spring-boot-starter-jetty",
				"spring-boot-starter-reactor-netty",
				"spring-boot-starter-undertow",
			)
		},
		Apply: func(ctx context.Context, target *corev1.PodTemplateSpec, containerIdx int, metadata ImageMetadata) error {
			applicationProperties := GetSpringApplicationProperties(ctx)

			var k8sGracePeriodSeconds int64 = 30 // default k8s grace period is 30 seconds
			if target.Spec.TerminationGracePeriodSeconds != nil {
				k8sGracePeriodSeconds = *target.Spec.TerminationGracePeriodSeconds
			}
			target.Spec.TerminationGracePeriodSeconds = &k8sGracePeriodSeconds
			// allocate 80% of the k8s grace period to boot
			bootGracePeriodSeconds := int(math.Floor(0.8 * float64(k8sGracePeriodSeconds)))
			applicationProperties["server.shutdown.grace-period"] = fmt.Sprintf("%ds", bootGracePeriodSeconds)
			return nil
		},
	},
	&BasicConvention{
		Id: "spring-boot-web",
		Applicable: func(ctx context.Context, metadata ImageMetadata) bool {
			deps := GetDependenciesBOM(ctx)
			return deps.HasDependency("spring-boot") && deps.HasDependency("spring-web")
		},
		Apply: func(ctx context.Context, target *corev1.PodTemplateSpec, containerIdx int, metadata ImageMetadata) error {
			applicationProperties := GetSpringApplicationProperties(ctx)

			serverPort := applicationProperties.Default("server.port", "8080")
			port, err := strconv.Atoi(serverPort)
			if err != nil {
				return err
			}

			c := &target.Spec.Containers[containerIdx]

			if name, cp := findContainerPort(target.Spec, int32(port)); cp == nil {
				c.Ports = append(c.Ports, corev1.ContainerPort{
					ContainerPort: int32(port),
					Protocol:      corev1.ProtocolTCP,
				})
			} else if name != c.Name {
				// port is in use by a different container
				return fmt.Errorf("desired port %s is in use by container %q, set 'server.port' boot property to an open port", serverPort, name)
			}

			return nil
		},
	},
	&BasicConvention{
		Id: "spring-boot-actuator",
		Applicable: func(ctx context.Context, metadata ImageMetadata) bool {
			deps := GetDependenciesBOM(ctx)
			return deps.HasDependency("spring-boot-actuator")
		},
		Apply: func(ctx context.Context, target *corev1.PodTemplateSpec, containerIdx int, metadata ImageMetadata) error {
			applicationProperties := GetSpringApplicationProperties(ctx)

			managementPort := applicationProperties.Default("management.server.port", applicationProperties["server.port"])
			managementBasePath := applicationProperties.Default("management.endpoints.web.base-path", "/actuator")
			managementScheme := corev1.URISchemeHTTP
			if applicationProperties["management.server.ssl.enabled"] == "true" {
				managementScheme = corev1.URISchemeHTTPS
			}

			managementUri := fmt.Sprintf("%s://:%s%s", strings.ToLower(string(managementScheme)), managementPort, managementBasePath)
			setAnnotation(target, "boot.spring.io/actuator", managementUri)

			return nil
		},
	},
	&BasicConvention{
		Id: "spring-boot-actuator-probes",
		Applicable: func(ctx context.Context, metadata ImageMetadata) bool {
			deps := GetDependenciesBOM(ctx)
			return deps.HasDependency("spring-boot-actuator")
		},
		Apply: func(ctx context.Context, target *corev1.PodTemplateSpec, containerIdx int, metadata ImageMetadata) error {
			deps := GetDependenciesBOM(ctx)
			applicationProperties := GetSpringApplicationProperties(ctx)

			if v := applicationProperties.Default("management.health.probes.enabled", "true"); v != "true" {
				// management health probes were deactivated by the user, skip
				return nil
			}

			managementBasePath := applicationProperties["management.endpoints.web.base-path"]
			managementPort, err := strconv.Atoi(applicationProperties["management.server.port"])
			if err != nil {
				return err
			}
			managementScheme := corev1.URISchemeHTTP
			if applicationProperties["management.server.ssl.enabled"] == "true" {
				managementScheme = corev1.URISchemeHTTPS
			}

			var livenessEndpoint, readinessEndpoint string
			if deps.HasDependencyConstraint("spring-boot-actuator", ">= 2.3.0-0") {
				livenessEndpoint = "/health/liveness"
				readinessEndpoint = "/health/readiness"
			} else {
				livenessEndpoint = "/info"
				readinessEndpoint = "/info"
			}

			c := &target.Spec.Containers[containerIdx]

			// define probes
			if c.LivenessProbe == nil {
				c.LivenessProbe = &corev1.Probe{}
			}
			if c.LivenessProbe.ProbeHandler == (corev1.ProbeHandler{}) {
				c.LivenessProbe.ProbeHandler = corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   managementBasePath + livenessEndpoint,
						Port:   intstr.FromInt(managementPort),
						Scheme: managementScheme,
					},
				}
			}
			if c.ReadinessProbe == nil {
				c.ReadinessProbe = &corev1.Probe{}
			}
			if c.ReadinessProbe.ProbeHandler == (corev1.ProbeHandler{}) {
				c.ReadinessProbe.ProbeHandler = corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   managementBasePath + readinessEndpoint,
						Port:   intstr.FromInt(managementPort),
						Scheme: managementScheme,
					},
				}
			}
			if c.StartupProbe == nil {
				// currently alpha in k8s 1.16+
				c.StartupProbe = &corev1.Probe{
					InitialDelaySeconds: 1,
					PeriodSeconds:       1,
					FailureThreshold:    120,
				}
			}
			if c.StartupProbe.ProbeHandler == (corev1.ProbeHandler{}) {
				c.StartupProbe.ProbeHandler = *c.LivenessProbe.ProbeHandler.DeepCopy()
			}

			return nil
		},
	},

	// service intents
	&SpringBootServiceIntent{
		Id:        "service-intent-mysql",
		LabelName: "services.conventions.carto.run/mysql",
		Dependencies: []string{
			"mysql-connector-java",
			"r2dbc-mysql",
		},
	},
	&SpringBootServiceIntent{
		Id:        "service-intent-postgres",
		LabelName: "services.conventions.carto.run/postgres",
		Dependencies: []string{
			"postgresql",
			"r2dbc-postgresql",
		},
	},
	&SpringBootServiceIntent{
		Id:        "service-intent-mongodb",
		LabelName: "services.conventions.carto.run/mongodb",
		Dependencies: []string{
			"mongodb-driver-core",
		},
	},
	&SpringBootServiceIntent{
		Id:        "service-intent-rabbitmq",
		LabelName: "services.conventions.carto.run/rabbitmq",
		Dependencies: []string{
			"amqp-client",
		},
	},
	&SpringBootServiceIntent{
		Id:        "service-intent-redis",
		LabelName: "services.conventions.carto.run/redis",
		Dependencies: []string{
			"jedis",
		},
	},
	&SpringBootServiceIntent{
		Id:        "service-intent-kafka",
		LabelName: "services.conventions.carto.run/kafka",
		Dependencies: []string{
			"kafka-clients",
		},
	},
	&SpringBootServiceIntent{
		Id:        "service-intent-kafka-streams",
		LabelName: "services.conventions.carto.run/kafka-streams",
		Dependencies: []string{
			"kafka-streams",
		},
	},
}
