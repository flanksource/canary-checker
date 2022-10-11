## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/restic.svg' style='height: 32px'/> Restic



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| accessKey | AccessKey access key id for connection with aws s3, minio, wasabi, alibaba oss | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) |  |
| caCert | CaCert path to the root cert. In case of self-signed certificates | string |  |
| checkIntegrity | CheckIntegrity when enabled will check the Integrity and consistency of the restic reposiotry | bool |  |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **maxAge** | MaxAge for backup freshness | string | Yes |
| name | Name of the check | string |  |
| **password** | Password for the restic repository | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **repository** | Repository The restic repository path eg: rest:https://user:pass@host:8000/ or rest:https://host:8000/ or s3:s3.amazonaws.com/bucket_name | string | Yes |
| secretKey | SecretKey secret access key for connection with aws s3, minio, wasabi, alibaba oss | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) |  |
