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
           query: "SELECT 1"
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
