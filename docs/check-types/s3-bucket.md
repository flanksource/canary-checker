## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/s3Bucket.svg' style='height: 32px'/> S3Bucket

This check will

- search objects matching the provided object path pattern
- check that latest object is no older than provided MaxAge value in seconds
- check that latest object size is not smaller than provided MinSize value in bytes.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: s3-bucket-check
     spec:
       interval: 30
       s3Bucket:
         # Check for any backup not older than 7 days and min size 25 bytes
         - bucket: tests-e2e-1
           accessKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_ACCESS_KEY_ID
           secretKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_SECRET_ACCESS_KEY
           region: "minio"
           endpoint: "http://minio.minio:9000"
           filter:
             regex: "(.*)backup.zip$"
           maxAge: 7d
           minSize: 25b
           usePathStyle: true
           skipTLSVerify: true
         # Check for any mysql backup not older than 7 days and min size 25 bytes
         - bucket: tests-e2e-1
           accessKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_ACCESS_KEY_ID
           secretKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_SECRET_ACCESS_KEY
           region: "minio"
           endpoint: "http://minio.minio:9000"
           filter:
             regex: "mysql\\/backups\\/(.*)\\/mysql.zip$"
           maxAge: 7d
           minSize: 25b
           usePathStyle: true
           skipTLSVerify: true
         # Check for any pg backup not older than 7 days and min size 50 bytes
         - bucket: tests-e2e-1
           accessKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_ACCESS_KEY_ID
           secretKey:
             valueFrom:
               secretKeyRef:
                 name: aws-credentials
                 key: AWS_SECRET_ACCESS_KEY
           region: "minio"
           endpoint: "http://minio.minio:9000"
           filter:
             regex: "pg\\/backups\\/(.*)\\/backup.zip$"
           maxAge: 7d
           minSize: 25b
           usePathStyle: true
           skipTLSVerify: true
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **accessKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **bucket** |  | string | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| endpoint |  | string |  |
| filter |  | [FolderFilter](#folderfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| maxAge | MaxAge the latest object should be younger than defined age | Duration |  |
| maxCount | MinCount the minimum number of files inside the searchPath | *int |  |
| maxSize | MaxSize of the files inside the searchPath | Size |  |
| minAge | MinAge the latest object should be older than defined age | Duration |  |
| minCount | MinCount the minimum number of files inside the searchPath | *int |  |
| minSize | MinSize of the files inside the searchPath | Size |  |
| name | Name of the check | string |  |
| objectPath | glob path to restrict matches to a subset | string |  |
| region |  | string |  |
| **secretKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| skipTLSVerify | Skip TLS verify when connecting to aws | bool |  |
| test |  | [Template](#template) |  |
| usePathStyle | Use path style path: http://s3.amazonaws.com/BUCKET/KEY instead of http://BUCKET.s3.amazonaws.com/KEY | bool |  |

