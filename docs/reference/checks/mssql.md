## MsSQL

This check will try to connect to a specified MsSQL database, run a query against it and verify the results.

??? example
     ```yaml
      apiVersion: canaries.flanksource.com/v1
      kind: Canary
      metadata:
        name: mssql-check
      spec:
        interval: 30
        spec:
          mssql:
            - connection: "server=mssql.default.svc;user id=$(username);password=$(password);port=1433;database=master"
              auth:
                username:
                  valueFrom: 
                    secretKeyRef:
                      name: mssql-credentials
                      key: USERNAME
                password:
                  valueFrom: 
                    secretKeyRef:
                      name: mssql-credentials
                      key: PASSWORD
              query: <insert-query>
              results: 1
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth | Username and password value, configMapKeyRef or SecretKeyRef for Postgres server | [Authentication](#authentication) |  |
| **connection** | Connection string to connect to the MsSQL server | string | Yes |
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
| **password** | Set password for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **username** | Set username for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes | 

## Template

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath | Specify JSON path for use in template| string |  |
| template | Specify jinja template for use | string |  |
| expr | Specify expression for use in template  | string |  |
| javascript | Specify javascript syntax for template | string |  |
