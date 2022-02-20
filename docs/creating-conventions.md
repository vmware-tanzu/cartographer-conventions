# <a id='Creating'></a> Creating a Convention Server

The Tanzu Application Platform enables developers to turn source code into a workload running in a container with a URL in minutes. In the process the [Convention Service](about.md). examines the workloads and matches them to conventions which describe changes or additions to make to. This lets the ops team roll in infrastructure or compliance opinions without adding to the workload of the application developer.

This document describes how to create conventions as well as a convention server to apply them.

### <a id='conventionservice'></a> About Convention Servers

A convention is used to to adapt or modify a [PodIntent](./reference/pod-intent.md) according to the type of an application. Conventions are defined by platform operators to automate the application of configuration and organizational best practices.

The convention is applied by the Convention Server. The server is called by Cartographer Conventions whenever a [PodIntent](./reference/pod-intent.md) is submitted.

1. <a id='create-1'></a>Take a look at the [convention template](TODO), which contains:

    ```shell
    server.go      # Defines the workload criteria, and actions to take when the criteria is met
    server.yaml    # Defines the kubernetes resources that make up the convention server
    ```
    
### <a id='conventionservice'></a> About Cartographer Conventions

Cartographer Conventions runs on a workload cluster as a Kubernetes deployment that runs a webhook which receives the workload [PodTemplateSpec](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) that defines how the workload should be run, as well as metadata.

Cartographer Conventions is the orchestrator of the Convention Servers. It sends to each of these servers that live in the current cluster, all the workload information. If the convention's criteria are met it enriches the [PodTemplateSpec](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) before returning it to the supply chain to be applied.

It also takes care of the reconciliation among conventions if the workload is updated.

## <a id='prereqs'></a>Before You Begin

There are a few things that will need to be done to create and install conventions:

+ The Kubectl CLI has been [installed](https://kubernetes.io/docs/tasks/tools/)
+ Tanzu Application Platform components have been installed on a k8s cluster [installation guide](https://docs.vmware.com/en/VMware-Tanzu-Application-Platform/0.2/tap-0-2/GUID-install-general.html)
+ The default supply chain is installed. See [Tanzu Network](https://network.tanzu.vmware.com/products/ootb-supply-chain-basic/).
+ Your kubeconfig context has been set to the prepared cluster `kubectl config use-context CONTEXT_NAME`
+ The ko CLI has been installed [on github](https://github.com/google/ko). These instructions use ko to build an image. If there is an existing image or build process, ko is optional.

_NOTE: this example covers developing conventions with [GOLANG](https://golang.org/) but it can be done in other languages by following the specs._
## <a id='server-behavior'></a> Define Convention Criteria and Behavior

The `server.go` file contains the configuration for the server as well as the logic the server applies when a workload matches the defined criteria. 
For example, adding a prometheus sidecar to web apps, or adding a `workload-type=spring-boot` label to any workload that has has metadata indicating that it is a spring boot app.  

**NOTE:** For this example, the package `model` is used to define [resources](./reference/README.md) types.

1. <a id='convention-1'></a>The example `server.go` sets up the `ConventionHandler` to ingest the webhook requests([PodConventionContext](./reference/pod-convention-context.md)) from Cartographer Conventions; at this point the handler only need to deal with the existing [`PodTemplateSpec`](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) and [`ImageConfig`](./reference/image-config.md).
   ```go
    ...
    import (
        corev1 "k8s.io/api/core/v1"
    ) 
    ...
    func ConventionHandler(template *corev1.PodTemplateSpec, images []model.ImageConfig) ([]string, error) {
        // Create custom conventions
    }
    ...
    ```

     Where:

     + `template` is the predefined [PodTemplateSpec](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) that the convention is going to modify.
     + `images` are the [`ImageConfig`](./reference/image-config.md) which will be used as reference to make decisions in the conventions. In this example the type was created within the `model` package

2. <a id='server-2'></a>The example `server.go` also configures the convention server to listen for requests

    ```go

        ...
        import (
            "context"
            "fmt"
            "log"
            "net/http"
            "os"
            ...
            "github.com/go-logr/logr"
            "github.com/go-logr/zapr"
            "go.uber.org/zap"
            ...
        )
        ...
        func main() {
            ctx := context.Background()
            port := os.Getenv("PORT")
            if port == "" {
                port = "9000"
            }
            zapLog, _ := zap.NewProductionConfig().Build()
            logger := zapr.NewLogger(zapLog)
            ctx = logr.NewContext(ctx, logger)
            http.HandleFunc("/", webhook.ServerHandler(convention.ConventionHandler))
            log.Fatal(webhook.NewConventionServer(ctx, fmt.Sprintf(":%s", port)))
        }
        ...

    ```
    Where:

    + `PORT` is a possible environment variable, for this example defined in the [`Deployment`](#install-deployment) 
    + `ServerHandler` is the *handler* defined in [last step](#create-1)
    + `NewConventionServer` is the function in charge of configure and create the *http webhook* server
    + `port` is the calculated port of the server to listen requests, it needs to match the [`Deployment`](#install-deployment) if the `PORT` variable is not defined in it
    + The `path` or pattern (default to `/`) is the convention server's default path if it is changed the it needs to be changed in the [`ClusterPodConvention`](#install-convention)

**Note:** The *Server Handler* (`func ConventionHandler(...)`) and the configure/start web server (`func NewConventionServer(...)`) are defined in Cartographer Conventions within the `webhook` package but a custom one can be used. In case of use the provided handler a logger of type [`logr.Logger`](https://pkg.go.dev/github.com/go-logr/logr#Logger) needs to be provided through the context by using `logr.NewContext(ctx, logger)` otherwise no log output will be generated.

3. Creating the *Server Handler* which handles the request from Cartographer Conventions with the [PodConventionContext](./reference/pod-convention-context.md) serialized to JSON.

    ```go
    package webhook
    ...
    func ServerHandler(conventionHandler func(template *corev1.PodTemplateSpec, images []model.ImageConfig) ([]string, error)) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            ...
            // Check request method
            ...
            // Decode the PodConventionContext
            podConventionContext := &model.PodConventionContext{}
            err = json.Unmarshal(body, &podConventionContext)
            if err != nil {
                w.WriteHeader(http.StatusBadRequest)
                return
            }
            // Validate the PodTemplateSpec and ImageConfig
            ...
            // Apply the conventions
            pts := podConventionContext.Spec.Template.DeepCopy()
            appliedConventions, err := conventionHandler(pts, podConventionContext.Spec.Images)
            if err != nil {
                w.WriteHeader(http.StatusInternalServerError)
                return
            }
            // Update the applied conventions and status with the new PodTemplateSpec
            podConventionContext.Status.AppliedConventions = appliedConventions
            podConventionContext.Status.Template = *pts
            // Return the updated PodConventionContext
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(podConventionContext)
        }
    }
    ...
    ```

4. Configure and start the web server by defining the `NewConventionServer` function which will start the server with the defined port and current context. The server will use the `.crt` and `.key` files to handle *TLS* traffic.

    ```go
    package webhook
    ...
    // Watch will handle the security by certificates
    type certWatcher struct {
        CrtFile string
        KeyFile string

        m       sync.Mutex
        keyPair *tls.Certificate
    }
    func (w *certWatcher) Load() error {
        // Creates a X509KeyPair from PEM encoded client certificate and private key.
        ...
    }
    func (w *certWatcher) GetCertificate() *tls.Certificate {
        w.m.Lock()
        defer w.m.Unlock()

        return w.keyPair
    }
    ...
    func NewConventionServer(ctx context.Context, addr string) error {
        // Define a health check endpoint to readiness and liveness probes
        http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
        })

        if err := watcher.Load(); err != nil {
            return err
        }
        // Defines the server with the TSL configuration
        server := &http.Server{
            Addr: addr,
            TLSConfig: &tls.Config{
                GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
                    cert := watcher.GetCertificate()
                    return cert, nil
                },
                PreferServerCipherSuites: true,
                MinVersion:               tls.VersionTLS13,
            },
            BaseContext: func(_ net.Listener) context.Context {
                return ctx
            },
        }
        go func() {
            <-ctx.Done()
            server.Close()
        }()

        return server.ListenAndServeTLS("", "")
    }
    ```
## <a id='targeting'></a> Define the convention behavior

Any property or value within the [PodTemplateSpec](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) or OCI image metadata associated with a workload can be used to define the criteria for applying conventions. The following are a few examples.  

### Matching Criteria By Labels or Annotations:

When using labels or annotations to define whether a convention should be applied, the server will check the [PodTemplateSpec](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) of workloads. 

   + PodTemplateSpec
   
        ```yaml
        ...
        template:
          metadata:
            labels:
              awesome-label: awesome-value
            annotations:
              awesome-annotation: awesome-value
        ...
        ```

   + Handler

        ```go
        package convention
        ...
        func conventionHandler(template *corev1.PodTemplateSpec, images []model.ImageConfig) ([]string, error) {
            c:= []string{}
            // This convention will be applied if a specific label is present
            if lv, le := template.Labels["awesome-label"]; le && lv == "awesome-value" {
                // DO COOl STUFF
                c = append(c, "awesome-label-convention")
            }
            // This convention will be applied if a specific annotation is present
            if av, ae := template.Annotations["awesome-annotation"]; ae && av == "awesome-value" {
                // DO COOl STUFF
                c = append(c, "awesome-annotation-convention")
            }
            
            return c, nil
        }
        ...
        ```
    
 Where:
 + `conventionHandler` is the *handler*
 + `awesome-label` is the **label** that we want to validate
 + `awesome-annotation` is the **annotation** that we want to validate
 + `awesome-value` is the value that must have the **label**/**annotation**

### <a id='EnvironmentVariables'></a>Matching Criteria By Environment Variables

When using environment variables to define whether the convention is applicable or not, it should be present in the [PodTemplateSpec](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec).[spec](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#PodSpec).[containers](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#Container)[*].[env](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#environment-variables). and we can validate the value.

   + PodTemplateSpec

        ```yaml
        ...
        template:
          spec:
            containers:
              - name: awesome-container
                env:
        ...
        ```

   + Handler

        ```go
        package convention
        ...
        func conventionHandler(template *corev1.PodTemplateSpec, images []model.ImageConfig) ([]string, error) {
            if len(template.Spec.Containers[0].Env) == 0 {
                template.Spec.Containers[0].Env = append(template.Spec.Containers[0].Env, corev1.EnvVar{
                    Name: "MY_AWESOME_VAR",
                    Value: "MY_AWESOME_VALUE",
                })
                return []string{"awesome-envs-convention"}, nil
            }
            return []string{}, nil
            ...
        }
        ```

### <a id='ImageMetadata'></a>Matching Criteria By Image Metadata

For each image contained within the PodTemplateSpec, Cartographer Conventions will fetch the OCI image metadata and known [`bill of materials (BOMs)`](reference/bom.md) providing it to the convention server as [`ImageConfig`](./reference/image-config.md). This metadata can be introspected to make decisions about how to configure the PodTemplateSpec.

## <a id='install'></a> Configure and Install the Convention Server

The `server.yaml` defines the Kubernetes components that will enable the convention server in the cluster, the next definitions are within the file.

1. <a id='install-namespace'></a>A `namespace` will be created for the convention server components and will have the required objects to run the server. It is used in the [`ClusterPodConvention`](#install-convention) section to indicates the controller where the server is.
    
    ```yaml
    ...
    ---
    apiVersion: v1
    kind: Namespace
    metadata:
      name: awesome-convention
    ---
    ...
    ```
    
2. <a id='install-cm'></a>A cert manager [`Issuer`](https://cert-manager.io/docs/concepts/issuer/), will be created to issue the cert needed for TLS communication. (Optional)

    ```yaml
    ...
    ---
    # The following manifests contain a self-signed issuer CR and a certificate CR.
    # More document can be found at https://docs.cert-manager.io
    apiVersion: cert-manager.io/v1
    kind: Issuer
    metadata:
      name: awesome-selfsigned-issuer
      namespace: awesome-convention
    spec:
      selfSigned: {}
    ---
    ...
    ```

3. <a id='install-cert'></a>A self-signed [`Certificate`](https://cert-manager.io/docs/concepts/certificate/) will be created. (Optional)

    ```yaml
    ...
    ---
    apiVersion: cert-manager.io/v1
    kind: Certificate
    metadata:
      name: awesome-webhook-cert
      namespace: awesome-convention
    spec:
      subject:
        organizations:
        - vmware
        organizationalUnits:
        - tanzu
      commonName: awesome-webhook.awesome-convention.svc
      dnsNames:
      - awesome-webhook.awesome-convention.svc
      - awesome-webhook.awesome-convention.svc.cluster.local
      issuerRef:
        kind: Issuer
        name: awesome-selfsigned-issuer
      secretName: awesome-webhook-cert
      revisionHistoryLimit: 10
    ---
    ...
    ```

4. <a id='install-deployment'></a>A Kubernetes `Deployment` will be created for the webhook to run from. The container port defined by the `Deployment` will be used by the [`Service`](#install-service) to expose server.

    ```yaml
    ...
    ---
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: awesome-webhook
      namespace: awesome-convention
    spec:
      replicas: 1
      selector:
        matchLabels:
        app: awesome-webhook
      template:
        metadata:
          labels:
            app: awesome-webhook
        spec:
          containers:
          - name: webhook
            # Set the prebuilt image of the convention or use ko to build an image from code
            # see https://github.com/google/ko 
            image: ko://awesome-repo/awesome-user/awesome-convention
          env:
          - name: PORT
            value: "8443"
          ports:
          - containerPort: 8443
            name: webhook
          livenessProbe:
            httpGet:
              scheme: HTTPS
              port: webhook
              path: /healthz
          readinessProbe:
            httpGet:
              scheme: HTTPS
              port: webhook
              path: /healthz
          volumeMounts:
          - name: certs
            mountPath: /config/certs
            readOnly: true
        volumes:
        - name: certs
          secret:
            defaultMode: 420
            secretName: awesome-webhook-cert
    ---
    ...
    ```

5.  <a id='install-service'></a>A Kubernetes `Service` to expose the convention deployment will also be created. For this example the exposed port is the default `443` but if it is changed, the [`ClusterPodConvention`](#install-convention) needs to be updated with the proper one. 

    ```yaml
    ...
    ---
    apiVersion: v1
    kind: Service
    metadata:
      name: awesome-webhook
      namespace: awesome-convention
      labels:
        app: awesome-webhook
    spec:
      selector:
        app: awesome-webhook
      ports:
        - protocol: TCP
          port: 443
          targetPort: webhook
    ---
    ...
    ```
6. <a id='install-convention'></a>Finally, the [`ClusterPodConvention`](./reference/cluster-pod-convention.md) will add the convention to the cluster to make it available for the Cartographer Conventions

    ```yaml
    ...
    ---
    apiVersion: conventions.carto.run/v1alpha1
    kind: ClusterPodConvention
    metadata:
      name: awesome-convention
      annotations:
        conventions.carto.run/inject-ca-from: "awesome-convention/awesome-webhook-cert"
    spec:
      webhook:
        clientConfig:
          service:
            name: awesome-webhook
            namespace: awesome-convention
            # path: "/" # default
            # port: 443 # default
    ```

**_Optional_**: Only needed if self-signed certificate is being used. Otherwise, check the cert-manager documentation.

## How to Deploy a Convention Server

1. Build and Install the Convention

    + If the convention needs to be built and deployed, use the [ko](https://github.com/google/ko) tool to do so, it will compile yout _go_ code into a docker image and push it to the registry(`KO_DOCKER_REGISTRY`).
    
        ```bash
        ko apply -f dist/server.yaml
        ```

    + If a different tool is being used to build the image, the configuration can be also be applied using either `kubectl` or `kapp` setting the correct image in the [`Deployment`](#install-convention) descriptor.
    
       kubectl
       
        ```bash
        kubectl apply -f server.yaml
        ```
        
       kapp
       
        ```bash
        kapp deploy -y -a awesome-convention -f server.yaml 
        ```

2. Verify the Convention Server
To check the status of the convention server, check for the running convention pods:

    + If the server is running, `kubectl get all -n awesome-convention` will return something like ...
    
        ```text
        NAME                                       READY   STATUS    RESTARTS   AGE
        pod/awesome-webhook-1234567890-12345       1/1     Running   0          8h

        NAME                          TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)   AGE
        service/awesome-webhook       ClusterIP   10.56.12.49   <none>        443/TCP   28h

        NAME                                  READY   UP-TO-DATE   AVAILABLE   AGE
        deployment.apps/awesome-webhook       1/1     1            1           28h

        NAME                                             DESIRED   CURRENT   READY   AGE
        replicaset.apps/awesome-webhook-1234563213       0         0         0       23h
        replicaset.apps/awesome-webhook-5b79d5cb59       0         0         0       28h
        replicaset.apps/awesome-webhook-5bf557c9f8       1         1         1       20h
        replicaset.apps/awesome-webhook-77c647c987       0         0         0       23h
        replicaset.apps/awesome-webhook-79d9c6f74c       0         0         0       23h
        replicaset.apps/awesome-webhook-7d9d667b8d       0         0         0       9h
        replicaset.apps/awesome-webhook-8668664d75       0         0         0       23h
        replicaset.apps/awesome-webhook-9b6957476        0         0         0       24h
        ```

    + To verify the conventions are being applied, check the `PodIntent` of a workload that matches the convention criteria.
    
        ```bash
        kubectl -o yaml get podintents.conventions.carto.run awesome-app
        ```

        ```yaml
        apiVersion: conventions.carto.run/v1alpha1
        kind: PodIntent
        metadata:
          creationTimestamp: "2021-10-07T13:30:00Z"
          generation: 1
          labels:
            app.kubernetes.io/component: intent
            carto.run/cluster-supply-chain-name: awesome-supply-chain
            carto.run/cluster-template-name: convention-template
            carto.run/component-name: config-provider
            carto.run/template-kind: ClusterConfigTemplate
            carto.run/workload-name: awesome-app
            carto.run/workload-namespace: default
          name: awesome-app
          namespace: default
        ownerReferences:
        - apiVersion: carto.run/v1alpha1
          blockOwnerDeletion: true
          controller: true
          kind: Workload
          name: awesome-app
          uid: "********"
        resourceVersion: "********"
        uid: "********"
        spec:
        imagePullSecrets:
          - name: registry-credentials
            serviceAccountName: default
            template:
              metadata:
                annotations:
                  developer.conventions/target-containers: workload
                labels:
                  app.kubernetes.io/component: run
                  app.kubernetes.io/part-of: awesome-app
                  carto.run/workload-name: awesome-app
              spec:
                containers:
                - image: awesome-repo.com/awesome-project/awesome-app@sha256:********
                  name: workload
                  resources: {}
                  securityContext:
                  runAsUser: 1000
        status:
          conditions:
          - lastTransitionTime: "2021-10-07T13:30:00Z"
            status: "True"
            type: ConventionsApplied
          - lastTransitionTime: "2021-10-07T13:30:00Z"
            status: "True"
            type: Ready
        observedGeneration: 1
        template:
          metadata:
            annotations:
              awesome-annotation: awesome-value
              conventions.carto.run/applied-conventions: |-
                awesome-label-convention
                awesome-annotation-convention
                awesome-envs-convention
                awesome-image-convention
                developer.conventions/target-containers: workload
            labels:
              awesome-label: awesome-value
              app.kubernetes.io/component: run
              app.kubernetes.io/part-of: awesome-app
              carto.run/workload-name: awesome-app
              conventions.carto.run/framework: go
          spec:
            containers:
            - env:
              - name: MY_AWESOME_VAR
                value: "MY_AWESOME_VALUE"
              image: awesome-repo.com/awesome-project/awesome-app@sha256:********
              name: workload
              ports:
                - containerPort: 8080
                  protocol: TCP
              resources: {}
              securityContext:
                runAsUser: 1000
        ```

## <a id='next-steps'></a> Next Steps

Keep Exploring:

+ Try to use different matching criteria for the conventions or enhance the supply chain with multiple conventions


