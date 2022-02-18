# PodIntent

`PodIntent` applies conventions to a workload. The `.spec.template`'s PodTemplateSpec is enriched by the conventions and exposed as the `.status.template`s PodTemplateSpec. A log of which sources and conventions applied is captured with the `conventions.carto.run/applied-conventions` annotation on the PodTemplateSpec.

```yaml
apiVersion: conventions.carto.run/v1alpha1
kind: PodIntent
metadata:
  name: sample
spec:
  template:
    spec:
      containers:
      - name: workload
        image: ubuntu
```

The `PodIntent`'s [`CRD`](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) is provided with the `Convention Service` under the group `conventions.carto.run` and is currently in version `v1alpha1`.