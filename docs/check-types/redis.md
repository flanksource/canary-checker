## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/redis.svg' style='height: 32px'/> Redis

??? example
    ```yaml
    apiVersion: canaries.flanksource.com/v1
    kind: Canary
    metadata:
      name: redis-check
    spec:
      interval: 30
      redis:
        - addr: "redis.default.svc:6379"
          name: redis-check
          auth:
            username:
              valueFrom:
                secretKeyRef:
                  name: redis-credentials
                  key: USERNAME
            password:
              valueFrom:
                secretKeyRef:
                  name: redis-credentials
                  key: PASSWORD
          db: 0
          description: "The redis check"
    ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **addr** |  | string | Yes |
| auth |  | *[Authentication](#authentication) |  |
| **db** |  | int | Yes |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
