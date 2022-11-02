# Cartographer Conventions Changelog


## v0.2.2

- [FIX] Rebuild binary with latest paketobuildpacks/run-jammy-tiny image in response to CVE-2022-3602 and CVE-2022-3786: vulnerabilities in OpenSSL 3.0.x

## v0.2.1

- [FIX] Updates base image paketobuildpacks/run-jammy-tiny:latest
- [FIX] Modified controller to a metadata-only watch/cache for secrets

## v0.2.0

- [FEATURE] Add an optional selectorTarget field on the ClusterPodConvention resource to specify label source for ClusterPodConvention matchers (#158).
- [DOCS] Add details about the newly added optional field `selectorTarget`(#172).

## v0.1.0

Initial Release
