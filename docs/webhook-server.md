# Convention Webhook Server

Convention Webhook server defines function to enrich PodTemplateSpec by applying set of conventions. 
 
Convention Webhook are sent a POST request, with `Content-Type: application/json`, with an PodConventionContext API object in the `webhooks.conventions.carto.run` API group serialized to JSON as the body.

Convention webhooks server respond with a 200 HTTP status code, `Content-Type: application/json`, and a body containing an PodConventionContext API object in the `webhooks.conventions.carto.run` API group.
* PodConventionContext API Object path `status.template` populated with the enriched PodTemplatesSpec with the convention applied.
* PodConventionContext API Object path `status.appliedConventions` populated with name of conventions applied.

Example of request to convention webhook (converted from JSON to YAML for readability)

```yaml
apiVersion: webhooks.conventions.carto.run/v1alpha1
kind: PodConventionContext
metadata:
  name: sample
spec:
  imageConfig:
  - image: ubuntu
    boms:
    - name: cnb-app:.../sbom.cdx.json
      raw: ...
    config:
      architecture: amd64
      os: linux
      container: ce366f29d6016d41ac378597493aa148c8dcd456dd5d996dfa8fa4c38b09fd2e
      config:
        Cmd:
        - "/bin/bash"
        Env:
        - "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
        Image: sha256:4e4bc990609ed865e07afc8427c30ffdddca5153fd4e82c20d8f0783a291e241
        <truncated>
  template:
    spec:
      containers:
      - name : workload
        image: ubuntu
```

Example of response from convention webhook (converted from JSON to YAML for readability)

```yaml
apiVersion: webhooks.conventions.carto.run/v1alpha1
kind: PodConventionContext
metadata:
  name: sample
spec: <truncated>
status:
  template:
    spec:
      containers:
      - name : spring-conventions
        image: ubuntu
      - name: prometheus-collector
        image: prom/prometheus:v2.1.0
        ports:
        - containerPort: 9090
          protocol: TCP
  appliedConventions:
  - promtheus
```

The server can be written in any language or framework that can run a TLS enabled web server. [Sample servers](../README.md#samples) are written in Golang and run as bare Kubernetes Deployments, with certificates provisioned by cert-manager.

The webhook server is made known to Cartographer Conventions via the [`ClusterPodConvention`](reference/cluster-pod-convention.md) resource, specifically the [WebhookClientConfig](https://pkg.go.dev/k8s.io/api/admissionregistration/v1#WebhookClientConfig) defined at `.spec.webhook.clientConfig`. The client config allows specify the webhook either via a URL or a ServiceReference. If the server is running within the same cluster, using the ServiceReference is strongly recommended. The server must be secured using a valid TLS certificate matching the hostname used to connect to the service. If the cluster does not already trust the certificate authority signing the server's certificate the CA bundle can also be included in the client config.

If using cert-manager to provision the certificate for the webhook sever, instead of referencing the CA bundle manually, a reference to the cert-manager Certificate resource can be defined as an annotation on the ClusterPodConvention. For example `conventions.carto.run/inject-ca-from: my-namespace/my-certificate`.

---

[back](./README.md)
