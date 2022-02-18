# Installing Release Bundles

## Preparation
### CLI Requirements
- [imgpkg](https://carvel.dev/imgpkg/) relocates a bundle into a local registry
- [kbld](https://carvel.dev/kbld/) rewrites image references in k8s yaml to pull from a relocated registry
- [kapp](https://carvel.dev/kapp/) for deploys and verifies k8s yaml

> All 3 CLIs can be easily installed via brew: `brew tap vmware-tanzu/carvel && brew install imgpkg kbld kapp`.

### Cluster Requirements
- [cert-manager](https://cert-manager.io/docs/installation/kubernetes/) must be installed for the Cartographer Conventions

## Installation

Download your desired release from [the releases page](https://github.com/vmware-tanzu/dap-framework/releases).

To install the bundle, you'll first need to relocate it to a docker registry that you can access from your cluster.

```sh
export CARTOGRAPHER_CONVENTIONS_VERSION=v0.0.0-test # update when we cut the initial release
imgpkg copy --tar ~/Downloads/cartographer-conventions-controller-bundle-${CARTOGRAPHER_CONVENTIONS_VERSION}.tar --to-repo ${DOCKER_REPO?:Required}/cartographer-conventions-controller-bundle
```

Then, pull down the yaml you'll need for the installation and cd into the bundle:

```sh
rm -rf ./cartographer-conventions-controller-bundle # imgpkg will create this for us
imgpkg pull -b ${DOCKER_REPO?:Required}/cartographer-conventions-controller-bundle -o ./cartographer-conventions-controller-bundle
cd ./cartographer-conventions-controller-bundle
```

Create a namespace to deploy components

```sh
kubectl create ns cartographer-conventions-system
```

Skip this step if images are accessible from the Kubernetes Cluster. If the images are relocated to a private registry, then update the registry credentials secret to the `default` serviceAccount in the namespace:

```sh
kubectl patch serviceaccount -n cartographer-conventions-system default -p '{"imagePullSecrets": [{"name": "'${REPLACE_WITH_REGISTRY_CREDS_SECRET_NAME}'"}]}'
```

Optional: Trust additional certificate authorities certificate

If a PodIntent references an image in a registry whose certificate was ***not*** signed by a Public Certificate Authority (CA), a certificate error `x509: certificate signed by unknown authority` will occur while applying conventions. To trust additional certificate authorities include the PEM encoded CA certificates in a file and set following environment variable to the location of that file.

```sh
CA_DATA=path/to/certfile # a PEM-encoded CA certificate
```

**Note** :  Follow [Kubernetes official documentation](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#create-a-secret-by-providing-credentials-on-the-command-line) on how to create a Secret to pull images from a private Docker registry or repository.

With the images relocated and the unpacked bundle as your current working directory, deploy Cartographer Conventions:

```sh
kapp deploy -a controller -n cartographer-conventions-system -f <(ytt -f config/cartogrpaher-conventions.yaml -f ca-overlay.yaml --data-value-file ca_cert_data=${CA_DATA:-ca.pem} | kbld -f .imgpkg/images.yml -f -)
```

> If kapp fails to find 'cert-manager.io/v1/Certificate', go [back](#cluster-requirements) and install [cert-manager](https://cert-manager.io/docs/installation/kubernetes/)

## Validation

At this point, you should have a working Convention Service installation. Try out the [convention server](../samples/convention-server) sample to take it for a spin, and review the [main README](../README.md) to learn more about Convention Service.

---

<!-- markdown-toc start - Don't edit this section. Run M-x markdown-toc-refresh-toc -->
**Table of Contents**

- [Installing Release Bundles](#installing-release-bundles)
  - [Preparation](#preparation)
    - [CLI Requirements](#cli-requirements)
    - [Cluster Requirements](#cluster-requirements)
  - [Installation](#installation)
  - [Validation](#validation)

<!-- markdown-toc end -->

[back](./README.md)
