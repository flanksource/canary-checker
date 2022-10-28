## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/mongodb.svg' style='height: 32px'/> MongoDB

The Mongo check tries to connect to a specified Mongo Database to ensure connectivity.

??? example
     ```yaml
      apiVersion: canaries.flanksource.com/v1
      kind: Canary
      metadata:
        name: mongo-check
      spec:
        interval: 30
        spec:
          mongodb:
            - connection: mongodb://$(username):$(password)@mongo.default.svc:27017/?authSource=admin
              description: mongo ping
              auth:
                username:
                  valueFrom: 
                    secretKeyRef:
                      name: mongo-credentials
                      key: USERNAME
                password:
                  valueFrom: 
                    secretKeyRef:
                      name: mongo-credentials
                      key: PASSWORD
              dns:
                - query: mongo.default.svc
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth | Username and password value, configMapKeyRef or SecretKeyRef for Mongo server | [Authentication](#authentication) |  |
| **connection** | Connection string to connect to the Mongo server | string | Yes |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |

---
# Scheme Reference
## Authentication

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **password** | Set password for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **username** | Set username for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes | 