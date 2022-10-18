## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/gcsBucket.svg' style='height: 32px'/> GCSBucket

??? example
     ```yaml
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **bucket** |  | string | Yes |
| **credentials** |  | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| **endpoint** |  | string | Yes |
| filter |  | [FolderFilter](#folderfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| maxAge | MaxAge the latest object should be younger than defined age | Duration |  |
| maxCount | MinCount the minimum number of files inside the searchPath | *int |  |
| maxSize | MaxSize of the files inside the searchPath | Size |  |
| minAge | MinAge the latest object should be older than defined age | Duration |  |
| minCount | MinCount the minimum number of files inside the searchPath | *int |  |
| minSize | MinSize of the files inside the searchPath | Size |  |
| name | Name of the check | string |  |
| test |  | [Template](#template) |  |
