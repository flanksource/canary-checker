## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/postgres.svg' style='height: 32px'/> Postgres

This check will try to connect to a specified Postgresql database, run a query against it and verify the results.

??? example
     ```yaml
      apiVersion: canaries.flanksource.com/v1
      kind: Canary
      metadata:
        name: postgres-check
      spec:
        interval: 30
        spec:
          postgres:
            - connection: "postgres://$(username):$(password)@postgres.default.svc:5432/postgres?sslmode=disable"
              auth:
                username:
                  valueFrom: 
                    secretKeyRef:
                      name: postgres-credentials
                      key: USERNAME
                password:
                  valueFrom: 
                    secretKeyRef:
                      name: postgres-credentials
                      key: PASSWORD
              query: SELECT current_schemas(true)
              display:
                template: |
                  {{- range $r := .results.rows }}
                  {{- $r.current_schemas}}
                  {{- end}}
              results: 1
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth | username and password value, configMapKeyRef or SecretKeyRef for Postgres server | [Authentication](#authentication) |  |
| **connection** | connection string to connect to the server | string | Yes |
| description | Description for the check | string |  |
| display | Template to display query results in text (overrides default bar format for UI) | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **query** | query that needs to be executed on the server | string | Yes |
| **results** | Number rows to check for | int | Yes |
| test | Template to test the result against | [Template](#template) |  |

---
# Scheme Reference
## Authentication



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **password** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **username** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |


## Template



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath |  | string |  |
| template |  | string |  |
