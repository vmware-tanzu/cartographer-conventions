# Dumper convention webserver 

The dumper is a convention server that "dumps" the `PodConventionContext` request made by the convention controller to stdout. It can be useful for gather OCI image metadata and SBOMs as a convention will receive the request.

## Trying out

Build and run the convention server:

```sh
# either, from source:
ko apply -f server.yaml

# or, from a release distribution:
kubectl create -f <(kbld -f server.yaml -f ../.imgpkg/images.yml)
```

To verify the dumped values in the logs

```sh
kubectl logs -n dumper-conventions -l app=webhook --tail 1000
```

Depending on the size of the `PodConventionContext` resources, this command may collect multiple requests, or only a portion of a single request. Adjust the number of lines collected as appropriate.
