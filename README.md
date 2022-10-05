# Cartographer Conventions <!-- omit in toc -->

Conventions allow an operator to define cross cutting behavior that are directly relevant to the developer's intent. Conventions reduce the amount of manual configuration required to run applications on Kubernetes effectively.

- [Pre-requisites](#pre-requisites)
- [Install](#install)
  - [From Source](#from-source)
- [Install on AWS](#running-cartographer-convention-on-an-aws)
- [Samples](#samples)
- [Contributing](#contributing)
- [License](#license)

## Pre-requisites

This project requires access to a [container registry](https://docs.docker.com/registry/introduction/) for fetching image metadata. It will not work for images that have bypassed a registry by loading directly into a local daemon.

## Install

### From Source

We use [Golang 1.19+](https://golang.org) and [`ko`](https://github.com/google/ko) to build the controller, and recommend [`kapp`](https://get-kapp.io) to deploy.

1. Install cert-manager

   ```sh
   kapp deploy -n kube-system -a cert-manager -f dist/third-party/cert-manager.yaml
   ```

2. Create a namespace to deploy components, if it doesn't already exist

   ```sh
   kubectl create ns cartographer-system
   ```

3. Optional: Trust additional certificate authorities certificate
  
    If a PodIntent references an image in a registry whose certificate was ***not*** signed by a Public Certificate Authority (CA), a certificate error `x509: certificate signed by unknown authority` will occur while applying conventions. To trust additional certificate authorities include the PEM encoded CA certificates in a file and set following environment variable to the location of that file.

    ```sh
    CA_DATA=path/to/certfile # a PEM-encoded CA certificate
    ```

4. Build and install Cartographer Conventions

    ```sh
    kapp deploy -n cartographer-system -a conventions \
      -f <( \
        ko resolve -f <( \
          ytt \
            -f dist/cartographer-conventions.yaml \
            -f dist/ca-overlay.yaml \
            --data-value-file ca_cert_data=${CA_DATA:-dist/ca.pem} \
          ) \
      )
    ```

    Note: you'll need to `export KO_DOCKER_REPO=<ACCESSIBLE_DOCKER_REPO>` such that `ko` can push to the repository and your cluster can pull from it. Visit [the ko README](https://github.com/ko-build/ko) for more information.

## Running cartographer convention on an AWS cluster

In order to [attach an IAM role](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) to the service account that the controller uses, provide the role [arn](https://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html) during installation phase.

  ```sh
  kapp deploy -n cartographer-system -a conventions \
    -f <( \
      ko resolve -f <( \
        ytt \
          -f dist/cartographer-conventions.yaml \
          -f dist/ca-overlay.yaml \
          -f dist/sa-arn-annotation-overlay.yaml \
          --data-value-file ca_cert_data=${CA_DATA:-dist/ca.pem} \
          --data-value aws_iam_role_arn="eks.amazonaws.com/role-arn: arn:aws:iam::133523324:role/role_name"
        ) \
    )
  ```

The service account `cartographer-conventions-controller-manager` would have the role arn added as annotation

  ```sh
  apiVersion: v1
  kind: ServiceAccount
  metadata:
    labels:
      app.kubernetes.io/component: conventions
    name: cartographer-conventions-controller-manager
    namespace: cartographer-system
    annotations:
      eks.amazonaws.com/role-arn: 'eks.amazonaws.com/role-arn: arn:aws:iam::133523324:role/role_name'
  ```

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
