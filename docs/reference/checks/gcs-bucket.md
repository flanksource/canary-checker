## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/gcsBucket.svg' style='height: 32px'/> GCSBucket

This check:

* Searches objects matching the provided object path pattern
* Checks that latest object is no older than provided maxAge value in seconds
* Checks that latest object size is not smaller than provided minSize value in bytes

**Note:** For working with the `folder` check, the bucket name needs to be prefixed with `gcs://`

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
      kind: Canary
      metadata:
        name: gcs-bucket check
      spec:
        interval: 30
        spec:
          folder:
           - description: gcs auth test
             path: gcs://somegcsbucket
             gcpConnection:
               endpoint: <gcs-bucket-endpoint>
               credentials:
                 valueFrom:
                   configMapKeyRef:
                     key: canary-checker-df017acc453c.json
                     name: sa
             minAge: 1m
             maxAge: 5h
             minSize: 2M
             maxCount: 2
             minCount: 5
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **bucket** |  | string | Yes |
| **credentials** |  | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| **endpoint** | GCS Bucket URL endpoint | string | Yes |
| filter |  | [FolderFilter](#folderfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| maxAge | MaxAge the latest object should be younger than defined age | Duration |  |
| maxCount | MinCount the minimum number of files inside the searchPath | *int |  |
| maxSize | MaxSize of the files inside the searchPath | Size |  |
| minAge | MinAge the latest object should be older than defined age | Duration |  |
| minCount | MinCount the minimum number of files inside the searchPath | *int |  |
| minSize | MinSize of the files inside the searchPath | Size |  |
| name | Name of the check | string |  |
| path | Path to the object, needs to be prefixed with the protocol. See example above | string | Yes
| gcpConnection | Creates connection to GCS bucket via credentials set | Object | Yes
| test |  | [Template](#template) |  |

---
# Scheme Reference
## FolderFilter



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| maxAge |  | Duration |  |
| maxSize |  | Size |  |
| minAge |  | Duration |  |
| minSize |  | Size |  |
| regex |  | string |  |

## Template



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath |  | string |  |
| template |  | string |  |