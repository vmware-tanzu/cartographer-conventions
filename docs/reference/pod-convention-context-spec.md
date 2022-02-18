# PodConventionContextSpec

The pod convention context specification is a wrapper of the [`PodTemplateSpec`](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) and the [`ImageConfig`](image-config.md) that is provided in the request body of the server and represents the original [`PodTemplateSpec`](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec).

```json
{
"template": {
    "metadata": {
        ...
    },
    "spec": {
        ...
    }
},
"imageConfig": [
    {
        "name": "oci-image-name",
        "boms": [
            {
                "name": "cnb-app:/layers/sbom/launch/paketo-buildpacks_executable-jar/sbom.cdx.json",
                "raw": ...,
            },
            ...
        ],
        "config": { ... },
    },
    ...
]
```