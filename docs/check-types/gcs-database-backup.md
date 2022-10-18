## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/databasebackupcheck.svg' style='height: 32px'/> DatabaseBackup

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: database-backup-check
     spec:
       interval: 60
       databaseBackup:
         - maxAge: 6h
           GCP:
             project: google-project-name
             instance: cloudsql-instance-name
     ```        

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| gcp |  | *[GCPDatabase](#gcpdatabase) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| labels | Labels for the check | Labels |  |
| maxAge |  | Duration |  |
| **name** | Name of the check | string | Yes |
| test |  | [Template](#template) |  |
| transform |  | [Template](#template) |  |
