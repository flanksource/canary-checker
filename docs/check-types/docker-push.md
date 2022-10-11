## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/dockerPush.svg' style='height: 32px'/> DockerPush

DockerPush check will try to push a Docker image to specified registry.
/*
??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: docker-push0-pass
     spec:
       interval: 30
       dockerPush:
         - image: ttl.sh/flanksource-busybox:1.30
           auth:
             username:
               value: test
             password:
               value: pass
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | *[Authentication](#authentication) |  |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **image** |  | string | Yes |
| name | Name of the check | string |  |

