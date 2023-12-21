# Sample springboot convention webserver

The image contains a simple Go webserver `server.go`, that will by default, listens on port `9000` and expose a service at `/`.

The webserver can only be called with request body of type `PodConventionContext` of APIGroup `webhooks.conventions.carto.run`. If not, server responds with 400 status code.
When called with correct request body type, the server emits a response of object type `PodConventionContext` of APIGroup `webhooks.conventions.carto.run`. This spring sample webserver adds annotation `boot.spring.io/version` with value {spring-boot-version} and add label `conventions.carto.run/framework` with value `spring-boot` if the container image has `spring-boot` dependency.

## Trying out

Build and run the convention server:

```
# either, from source:
ko apply -f server.yaml

# or, from a release distribution:
kubectl create -f <(kbld -f server.yaml -f ../.imgpkg/images.yml)
```

To verify the setup, add an Workload and check the status

```sh
kubectl create -f workload.yaml
```

```
kubectl get podintents.conventions.carto.run spring-sample -oyaml
```

If everything works correctly, the status will contain a transformed template that includes Spring Boot variables added by the convention server.

```yaml
apiVersion: conventions.carto.run/v1alpha1
kind: PodIntent
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"conventions.carto.run/v1alpha1","kind":"PodIntent","metadata":{"annotations":{},"name":"spring-sample","namespace":"test"},"spec":{"template":{"spec":{"containers":[{"image":"krashed843/tanzu-java-app@sha256:9f90358e4c4eff2255bab81e3fa6316418ef435465dbae3bba74a2262f7c227d","name":"workload"}]}}}}
  creationTimestamp: "2023-12-21T20:40:59Z"
  generation: 1
  name: spring-sample
  namespace: test
  resourceVersion: "7568605"
  uid: 3c263b9c-a586-4933-97a8-604a1badeccb
spec:
  serviceAccountName: default
  template:
    metadata: {}
    spec:
      containers:
      - image: krashed843/tanzu-java-app@sha256:9f90358e4c4eff2255bab81e3fa6316418ef435465dbae3bba74a2262f7c227d
        name: workload
        resources: {}
status:
  conditions:
  - lastTransitionTime: "2023-12-21T20:40:59Z"
    message: ""
    reason: Applied
    status: "True"
    type: ConventionsApplied
  - lastTransitionTime: "2023-12-21T20:40:59Z"
    message: ""
    reason: ConventionsApplied
    status: "True"
    type: Ready
  observedGeneration: 1
  template:
    metadata:
      annotations:
        boot.spring.io/actuator: http://:8080/actuator
        boot.spring.io/version: 2.7.15
        conventions.carto.run/applied-conventions: |-
          spring-sample/spring-boot
          spring-sample/spring-boot-web
          spring-sample/spring-boot-actuator
          spring-sample/spring-boot-actuator-probes
      labels:
        conventions.carto.run/framework: spring-boot
    spec:
      containers:
      - env:
        - name: JAVA_TOOL_OPTIONS
          value: -Dmanagement.endpoints.web.base-path=/actuator -Dmanagement.health.probes.enabled=true
            -Dmanagement.server.port=8080 -Dserver.port=8080
        image: index.docker.io/krashed843/tanzu-java-app@sha256:9f90358e4c4eff2255bab81e3fa6316418ef435465dbae3bba74a2262f7c227d
        livenessProbe:
          httpGet:
            path: /actuator/health/liveness
            port: 8080
            scheme: HTTP
        name: workload
        ports:
        - containerPort: 8080
          protocol: TCP
        readinessProbe:
          httpGet:
            path: /actuator/health/readiness
            port: 8080
            scheme: HTTP
        resources: {}
        startupProbe:
          failureThreshold: 120
          httpGet:
            path: /actuator/health/liveness
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 1
          periodSeconds: 1
```
