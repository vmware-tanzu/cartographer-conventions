# Convention Service <!-- omit in toc -->

## Overview

The convention service provides a means for people in operational roles to express
their hard-won knowledge and opinions about how applications should run on Kubernetes as a convention.
The convention service applies these opinions to fleets of developer workloads as they are deployed to the platform,
saving operator and developer time.

The service is comprised of two components:

* **Cartographer Conventions:**
  Cartographer Conventions provides the metadata to the Convention Server(s) and executes the updates Pod Template Spec(s) as per the Convention Server(s) requests.

* **The Convention Server:** 
  The Convention Server receives and evaluates metadata associated with a workload and
  requests updates to the Pod Template Spec associated with that workload. 
  There can be one or more Convention Servers for a single Cartographer Conventions instance.
  The convention service currently supports defining and applying conventions for pods.

## About Applying Conventions

The convention server uses criteria defined in the convention to determine
whether the configuration of a given workload should be changed.
The server receives the OCI metadata from Cartographer Conventions,
if the metadata meets the criteria defined by the convention server,
the conventions are applied.
It is also possible for a convention to apply to all workloads regardless of metadata.

### Applying Conventions Using Image Metadata

Conventions can be defined to target workloads using properties of their OCI metadata.

Conventions can use this information to only apply changes to the configuration of workloads
when they match specific critera (for example, spring boot or .net apps, or spring boot v2.3+).
Targeted conventions can ensure uniformity across specific workload types deployed on the cluster. 

All the metadata details of an image can be used when evaluating workloads,
and can be seen with the docker CLI command `docker image inspect IMAGE`.

> **Note**: Depending on how the image was built, metadata might not be available to reliably identify
the image type and match the criteria for a given convention server.
Images built with Cloud Native Buildpacks reliably include rich descriptive metadata.
Images built by some other process may not include the same metadata. 

### Applying Conventions without Using Image Metadata

Conventions can also be defined to apply to workloads without targeting build service metadata.
Examples of possible uses of this type of convention include appending a logging/metrics sidecar,
adding environment variables, or adding cached volumes.
These types of conventions can be a great way for operators to ensure infrastructure uniformity
across workloads deployed on the cluster while reducing developer toil.

> **Note**: Adding a sidecar alone does not magically make the log/metrics collection work.
  This requires collector agents to be already deployed and accessible from the Kuberentes cluster
and also configuring required access through RBAC policy.

## Convention Service Resources

There are two Kubernetes resources involved in the application of conventions to workloads.

### PodIntent

```yaml
apiVersion: conventions.carto.run/v1alpha1
kind: PodIntent
```

`PodIntent` applies conventions to a workload.
The `.spec.template`'s PodTemplateSpec is enriched by the conventions and exposed as the `.status.template`s PodTemplateSpec.
When a convention is applied, the name of the convention is added
as a `conventions.carto.run/applied-conventions` annotation on the PodTemplateSpec.

### ClusterPodConvention

```yaml
apiVersion: conventions.carto.run/v1alpha1
kind: ClusterPodConvention
```

`ClusterPodConvention` defines a way to connect to convention servers,
and it provides a way to apply a set of conventions to a PodTemplateSpec and the artifact metadata.
A convention typically focuses on a particular application framework, but may be cross cutting.
Applied conventions must be pure functions.

### How it works

#### API structure 

The `PodConventionContext` API object in the `webhooks.conventions.carto.run` API group is the structure used for both request and response from the convention server.

In PodConventionContext API resource:
* Object path `.spec.template` field defines the PodTemplateSpec to be enriched by conventions.
* Object path `.spec.imageConfig[]` field defines for each unique image referenced in the PodTemplateSpec:
    * `.image` the image reference, resolved to a canonical digested form
    * `.config` OCI metadata [ggcrv1.ConfigFile](https://pkg.go.dev/github.com/google/go-containerregistry@v0.7.0/pkg/v1#ConfigFile)
    * `.boms` list of Software Bill of Materials (SBOMs) found for the image

Following is an example of a `PodConventionContext` resource request that is received by the convention server. This object is generated for [Go language based image](https://github.com/paketo-buildpacks/samples/tree/main/go/mod) built with Cloud Native Paketo Buildpacks that uses Go mod for dependency management.

```yaml
---
apiVersion: webhooks.conventions.carto.run/v1alpha1
kind: PodConventionContext
metadata:
  name: sample # the name of the ClusterPodConvention
spec: # the request
  imageConfig: # one entry per image referenced by the PodTemplateSpec
  - image: sample/go-based-image
    boms:
    - name: cnb-app:.../sbom.cdx.json
      raw: ...
    config:
      entrypoint:
      - "/cnb/process/web"
      domainname: ""
      architecture: "amd64"
      image: "sha256:05b698a4949db54fdb36ea431477867abf51054abd0cbfcfd1bb81cda1842288"
      labels:
        "io.buildpacks.stack.distro.version": "18.04"
        "io.buildpacks.stack.homepage": "https://github.com/paketo-buildpacks/stacks"
        "io.buildpacks.stack.id": "io.buildpacks.stacks.bionic"
        "io.buildpacks.stack.maintainer": "Paketo Buildpacks"
        "io.buildpacks.stack.distro.name": "Ubuntu"
        "io.buildpacks.stack.metadata": `{"app":[{"sha":"sha256:ea4ec23266a3af1204fd643de0f3572dd8dbb5697a5ef15bdae844777c19bf8f"}], 
        "buildpacks":[{"key":"paketo-buildpac`...,
        "io.buildpacks.build.metadata": `{"bom":[{"name":"go","metadata":{"licenses":[],"name":"Go","sha256":"7fef8ba6a0786143efcce66b0bbfbfbab02afeef522b4e09833c5b550d7`...
  template:
    spec:
      containers:
      - name : workload
        image: helloworld-go-mod
```
#### PodConventionContext Structure 

Let's expand more on the image config present in `PodConventionContext`. Cartographer Conventions passes along this information for the image in good faith, the controller is not the source of the metadata and there is no guarantee that the information is correct.

The `config` field in the image config passes through the [OCI Image metadata](https://github.com/opencontainers/image-spec/blob/master/config.md) loaded from the registry for the image.

The `bom` field in the image config passes through the name and raw data found within the image. Conventions may parse the BOMs they want to inspect. There is no guarantee that an image will contain a BOM, that the BOM will be in a certain format.

#### Template Status

The enriched PodTemplateSpec is reflected at `.status.template`, which can be watched by the owner of the PodIntent resource.
The field `.status.appliedConventions` is populated with the names of any applied conventions.

The following is an example of a `PodConventionContext` response with the `.status` field populated.

```yaml
---
apiVersion: webhooks.conventions.carto.run/v1alpha1
kind: PodConventionContext
metadata:
  name: sample # the name of the ClusterPodConvention
spec: # the request
  imageConfig:
  template:
    <corev1.PodTemplateSpec>
status: # the response
  appliedConventions: # list of names of conventions applied
  - my-convention
  template:
  spec:
      containers:
      - name : workload
        image: helloworld-go-mod
```

## Chaining Multiple Conventions

Platform operators can define multiple `ClusterPodConventions` that can be applied to different types of workloads.
It is also possible for multiple conventions to apply to a workload. 

The `PodIntent` reconciler lists all `ClusterPodConvention` resources and applies them serially.
To ensure the consistency of enriched `podTemplateSpec`,
the list of ClusterPodConventions is sorted alphabetically by name before applying conventions.
If desired, strategic naming can be used to control the order in which the conventions are applied.

After the conventions are applied, the `Ready` status condition on the `PodIntent` resource is used to indicate
whether it is applied successfully or not.
A list of all applied conventions is stored under the annotation `conventions.carto.run/applied-conventions`.

