## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/containerdPull.svg' style='height: 32px'/> ContainerdPull

This check will try to pull a Docker image from specified registry using containerd and then verify its checksum and size.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: containerd-pull-check
     spec:
       interval: 30
       containerd:
         - image: docker.io/library/busybox:1.31.1
           expectedDigest: sha256:95cf004f559831017cdf4628aaf1bb30133677be8702a8c5f2994629f637a209
           expectedSize: 764556
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | [Authentication](#authentication) |  |
| description | Description for the check | string |  |
| expectedDigest |  | string |  |
| expectedSize |  | int64 |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **image** |  | string | Yes |
| name | Name of the check | string |  |
