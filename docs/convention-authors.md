# Convention Authors <!-- omit in toc -->

This document is targeted for authors of conventions. We will cover the following topics

- [Conventions using image metadata](#conventions-using-image-metadata)
- [Conventions without using image metadata](#conventions-without-using-image-metadata)
- [Convention contract](#convention-contract)
- [Chaining multiple conventions](#chaining-multiple-conventions)

## Conventions using image metadata

Conventions defined to enrich workloads that depend on OCI metadata. Images build with [Cloud Native buildpacks](https://buildpacks.io) are attached with metadata describing their content that is passed to the convention webserver along with workload template. Conventions can use this information to apply config mutations like [Spring Boot based conventions](../samples/spring-convention-server/README.md).

Examples for this type of convention include framework-based or language-based conventions. These types of conventions can ensure uniformity across workloads deployed on the cluster.

## Conventions without using image metadata

Conventions defined to enrich workloads without inspecting build service metadata. Examples for this type of service are appending logging/metrics sidecar, adding environment variables, adding cached volumes etc., These types of conventions can be a great way for operators to ensure infrastructure uniformity across workloads deployed on the cluster.

*Note*: Adding sidecar alone will not magically make the log/metrics collection work. This requires collector agents to be already deployed and accesible from the Kuberentes cluster and also configuring required access via RBAC policy.

There is another example of an [convention server](../samples/convention-server/README.md) that appends an environment variable `CONVENTION_SERVER` to all the containers.

## Convention contract

[`PodConventionContext`](./reference/pod-convention-context.md) API object in the `webhooks.conventions.carto.run` API group is the structure used for both request and reponse from convention server.


## Chaining multiple conventions

Platform operators can define multiple ClusterPodConventions that have the opportunity to advise different set of workloads. Each convention can return a transformed PodTemplateSpec along with a list of conventions applied.

PodIntent reconciler lists all ClusterPodConvention resources and applies them serially. To ensure consistent and reproducability of enriched podTemplateSpec, list of ClusterPodConventions are sorted by name before applying.

After the conventions are applied, the `Ready` status condition on PodIntent resource is used to indicate whether its applied successfully or not. A receipt of all applied conventions is stored under the annotation `conventions.carto.run/applied-conventions`
