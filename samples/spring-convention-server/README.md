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
  creationTimestamp: "2021-03-24T23:56:20Z"
  generation: 1
  name: spring-sample
  namespace: default
  resourceVersion: "6978954"
  selfLink: /apis/conventions.carto.run/v1alpha1/namespaces/default/podintents/sample
  uid: d8ade195-7f9d-4694-99b1-47b01052461b
spec:
  template:
    metadata: {}
    spec:
      containers:
      - image: scothis/petclinic:service-bindings-20200922
        name: workload
        resources: {}
status:
  conditions:
  - lastTransitionTime: "2021-03-24T23:56:21Z"
    status: "True"
    type: ConventionsApplied
  - lastTransitionTime: "2021-03-24T23:56:21Z"
    status: "True"
    type: Ready
  observedGeneration: 1
  template:
    metadata:
      annotations:
        boot.spring.io/version: 2.3.3.RELEASE
        conventions.carto.run/applied-conventions: spring-sample/spring-boot
      labels:
        conventions.carto.run/framework: spring-boot
    spec:
      containers:
      - image: scothis/petclinic:service-bindings-20200922
        name: workload
        resources: {}
```
