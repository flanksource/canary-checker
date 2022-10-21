## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/containerdPush.svg' style='height: 32px'/> ContainerdPush

This check will try to push a Docker image to a specified registry using containerd.


??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: containerd-push-check
     spec:
       interval: 30
       containerdPush:
         - name: ContainerdPush Check
           image: docker.io/library/busybox:1.31.1
           username: <insert-username>
           password: <insert-password>
             
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **image** |  | string | Yes |
| name | Name of the check | string |  |
| **password** |  | string | Yes |
| **username** |  | string | Yes |
