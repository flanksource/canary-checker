## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/dockerPull.svg' style='height: 32px'/> DockerPull

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: docker-pass
     spec:
       interval: 30
       docker:
         - image: docker.io/library/busybox:1.31.1@sha256:b20c55f6bfac8828690ec2f4e2da29790c80aa3d7801a119f0ea6b045d2d2da1
           expectedDigest: sha256:b20c55f6bfac8828690ec2f4e2da29790c80aa3d7801a119f0ea6b045d2d2da1
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | *[Authentication](#authentication) |  |
| description | Description for the check | string |  |
| **expectedDigest** |  | string | Yes |
| **expectedSize** |  | int64 | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **image** |  | string | Yes |
| name | Name of the check | string |  |
