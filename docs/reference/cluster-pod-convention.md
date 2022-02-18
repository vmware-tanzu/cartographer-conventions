# ClusterPodConvention

`ClusterPodConvention` is a `Cluster` scoped [custom](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) [_Kubernetes API Object_](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/) definition. It provides the definition of the convention server which will apply a set of conventions to a [`PodTemplateSpec`](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) and the artifact metadata.

[Webhook servers](../webhook-server.md) are the only way to define conventions currently.

```yaml
apiVersion: conventions.carto.run/v1alpha1
kind: ClusterPodConvention
metadata:
  name: base-convention
  annotations:
    conventions.carto.run/inject-ca-from: "convention-template/webhook-cert"
spec:
  priority: Normal
  webhook:
    clientConfig:
      service:
        name: webhook
        namespace: convention-template
        port: 443
        path: "/"
      # url: https://webhook.convention-template.svc.cluster.local/
      caBundle: "MIIDSjCCA...Mynb8Bndn/"
  selector:
    matchLabels:
      app: awesome-webhook
    matchExpressions:
      - key: app
        operator: In
        values:
        - awesome-webhook
    
```

The `ClusterPodConvention`'s [`CRD`](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) is provided with the `Convention Service` under the group `conventions.carto.run` and is currently in version `v1alpha1`.

The `spec` section (required) of the `ClusterPodConvention` defines the following properties:

+ `webhook`: The [`WebhookServer`](../webhook-server.md) definition which can be configured with a [`service`](https://kubernetes.io/docs/concepts/services-networking/service/) or a `url` setting an external webhook server, inside a `clientConfig` __required__ section.
  + [`service`](https://kubernetes.io/docs/concepts/services-networking/service/): The exposed application service which will be used to apply the conventions and can be configured with:
    + `name`(__required__): The name of the [`service`](https://kubernetes.io/docs/concepts/services-networking/service/).
    + `namespace`(__required__): The namespace in which the [`service`](https://kubernetes.io/docs/concepts/services-networking/service/) is.
    + `path`: The url path context of the [`service`](https://kubernetes.io/docs/concepts/services-networking/service/) to post the webhook request.
    + `port`: The port of the [`service`](https://kubernetes.io/docs/concepts/services-networking/service/) to post the webhook request.
  + `url`: The url of the external webhook server to post the request.
  + `caBundle`: The CA bundle of the webhook server to post the request.

+ `selectors`: Kubernetes selectors array to find the matching convention servers to apply the conventions.
  + `matchLabels`: Kubernetes labels to match the service to apply the conventions.
  + `matchExpressions`: Kubernetes expressions to match the service to apply the conventions.
+ `priority`: Defines the priority level of the `ClusterPodConvention`. Accepted values are:
  + `Early`: The `ClusterPodConvention` will be applied before others.
  + `Normal`: The `ClusterPodConvention` will be applied with no priority.
  + `Late`: The `ClusterPodConvention` will be applied after others.
  
  __Note__: If the `priority` is not specified, the `ClusterPodConvention` will be applied with `Normal` priority.
  
  __Note__: If more than one `ClusterPodConvention` is defined with the same `priority`, the `ClusterPodConvention`s will be applied in alphabetical order of the level.