# Cartographer Conventions <!-- omit in toc -->

Conventions allow an operator to define cross cutting behavior that are directly relevant to the developer's intent. Conventions reduce the amount of manual configuration required to run applications on Kubernetes effectively.

- [Pre-requisites](#pre-requisites)
- [Install](#install)
  - [From Source](#from-source)
- [Samples](#samples)
- [Contributing](#contributing)
- [License](#license)

## Pre-requisites

This project requires access to a [container registry](https://docs.docker.com/registry/introduction/) for fetching image metadata. It will not work for images that have bypassed a registry by loading directly into a local daemon.

## Install

### From Source

We use [Golang 1.18+](https://golang.org) and [`ko`](https://github.com/google/ko) to build the controller, and recommend [`kapp`](https://get-kapp.io) to deploy.

1. Install cert-manager

   ```sh
   kapp deploy -n kube-system -a cert-manager -f dist/third-party/cert-manager.yaml
   ```

2. Create a namespace to deploy components

   ```sh
   kubectl create ns cartographer-conventions-system
   ```

3. Optional: Trust additional certificate authorities certificate
  
    If a PodIntent references an image in a registry whose certificate was ***not*** signed by a Public Certificate Authority (CA), a certificate error `x509: certificate signed by unknown authority` will occur while applying conventions. To trust additional certificate authorities include the PEM encoded CA certificates in a file and set following environment variable to the location of that file.

    ```sh
    CA_DATA=path/to/certfile # a PEM-encoded CA certificate
    ```

4. Build and install Cartographer Conventions

    ```sh
    kapp deploy -n cartographer-conventions-system -a controller -f <(ytt -f dist/cartogrpaher-conventions.yaml -f dist/ca-overlay.yaml --data-value-file ca_cert_data=${CA_DATA:-dist/ca.pem} | ko resolve -f -)
    ```

    Note: you'll need to `export KO_DOCKER_REPO=<ACCESSIBLE_DOCKER_REPO>` such that `ko` can push to the repository and your cluster can pull from it. Visit [the ko README](https://github.com/google/ko/blob/master/README.md#usage) for more information.

## Samples

- [Convention Server](./samples/convention-server/)

  Apply custom conventions to workloads with a ClusterPodConvention pointing at a webhook convention server.

- [Spring Boot Conventions](./samples/spring-convention-server/)

  Apply custom conventions for Spring Boot workloads. This convention can detect if the workload is built from Spring Boot adding a label to the workload indicating the framework is `spring-boot`, and an annotation indicating the version of Spring Boot used.

- [Dumper Server](./samples/dumper-server/)

  Log the content of the webhook request to stdout. Useful for capturing the image metadata available to conventions.

## Contributing

The Cartographer project team welcomes contributions from the community. If you wish to contribute code and you have not signed our contributor license agreement (CLA), our bot will update the issue when you open a Pull Request. For any questions about the CLA process, please refer to our [FAQ](https://cla.vmware.com/faq). For more detailed information, refer to [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Refer to [LICENSE](LICENSE) for details.
