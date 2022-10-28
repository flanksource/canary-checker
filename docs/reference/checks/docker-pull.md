## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/dockerPull.svg' style='height: 32px'/> DockerPull

This check will try to pull a Docker image from a specified registry, verify it's checksum and size.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: docker-check
     spec:
       interval: 30
       docker:
         - image: docker.io/library/busybox:1.31.1@sha256:b20c55f6bfac8828690ec2f4e2da29790c80aa3d7801a119f0ea6b045d2d2da1
           expectedDigest: sha256:b20c55f6bfac8828690ec2f4e2da29790c80aa3d7801a119f0ea6b045d2d2da1
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth | Username and password value, configMapKeyRef or SecretKeyRef for registry | [Authentication](#authentication) |  |
| description | Description for the check | string |  |
| expectedDigest | Expected digest of the pulled image | string | Yes |
| expectedSize | Expected size of the pulled image | int64 | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **image** | Full path to image, including registry | string | Yes |
| name | Name of the check | string |  |

---
# Scheme Reference
## Authentication

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **password** | Set password for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **username** | Set username for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes | 