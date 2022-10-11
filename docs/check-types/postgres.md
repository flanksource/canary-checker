## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/postgres.svg' style='height: 32px'/> Postgres

This check will try to connect to a specified Postgresql database, run a query against it and verify the results.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: postgres-succeed
     spec:
       interval: 30
       postgres:
         - connection: "postgres://$(username):$(password)@postgres.default.svc:5432/postgres?sslmode=disable"
           auth:
             username:
               value: postgresadmin
             password:
               value: admin123
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
| auth |  | [Authentication](#authentication) |  |
| **connection** |  | string | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **query** |  | string | Yes |
| **results** | Number rows to check for | int | Yes |
| test |  | [Template](#template) |  |
