# Sample convention webserver 

The image contains a simple Go webserver `server.go`, that will by
default, listens on port `9000` and expose a service at `/`.

The webserver can only be called with request body of type `PodConventionContext` of APIGroup `webhooks.conventions.carto.run`. If not, server responds with 400 status code.
When called with correct request body type, the server emits a response of object type `PodConventionContext` of APIGroup `webhooks.conventions.carto.run` and for this sample, appends an environment variable.

## Trying out

Build and run the convention server:

```sh
# either, from source:
ko apply -f server.yaml

# or, from a release distribution:
kubectl create -f <(kbld -f server.yaml -f ../.imgpkg/images.yml)
```

To verify the setup, add an Workload and check the status

```sh
kubectl create -f workload.yaml
```

```sh
kubectl get podintents.conventions.carto.run sample -oyaml
```

If everything works correctly, the status will contain a transformed template that includes an environment variable added by the convention server.

```yaml
apiVersion: conventions.carto.run/v1alpha1
kind: PodIntent
metadata:
  creationTimestamp: "2021-01-20T02:06:33Z"
  generation: 1
  name: sample
  namespace: default
  resourceVersion: "6978954"
  selfLink: /apis/conventions.carto.run/v1alpha1/namespaces/default/podintents/sample
  uid: d8ade195-7f9d-4694-99b1-47b01052461b
spec:
  template:
    metadata: {}
    spec:
      containers:
      - image: ubuntu
        name: workload
        resources: {}
status:
  conditions:
  - lastTransitionTime: "2021-01-20T02:06:34Z"
    status: "True"
    type: ConventionsApplied
  - lastTransitionTime: "2021-01-20T02:06:34Z"
    status: "True"
    type: Ready
  observedGeneration: 1
  template:
    metadata:
      annotations:
        conventions.carto.run/applied-conventions: sample/add-env-var
    spec:
      containers:
      - image: ubuntu
        name: workload
        env:
        - name: CONVENTION_SERVER
          value: HELLO FROM CONVENTION
        resources: {}
```
