## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/databasebackupcheck.svg' style='height: 32px'/> DatabaseBackup

This check performs regular backups for you CloudSQL instance at specified intervals. 

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
           gcp:
             project: google-project-name
             instance: cloudsql-instance-name
     ```        

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| display | Template to display server response in text (overrides default bar format for UI) | [Template](#template) |  |
| gcp | Connect to GCP project and instance | *[GCPDatabase](#gcpdatabase) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| labels | Labels for the check | Labels |  |
| maxAge | Max age for backup allowed, eg. 5h30m | Duration |  |
| **name** | Name of the check | string | Yes |
| test | Template to test the result against | [Template](#template) |  |
| transform | Template to transform results to | [Template](#template) |  |

---
# Scheme Reference
## Template


| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath |  | string |  |
| template |  | string |  |

