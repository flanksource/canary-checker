## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/smb.svg' style='height: 32px'/> Smb

Smb check will connect to the given samba server with given credentials
find the age of the latest updated file and compare it with minAge
count the number of file present and compare with minCount if defined

??? example
     ```yaml
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **auth** |  | *[Authentication](#authentication) | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| domain | Domain... | string |  |
| filter |  | [FolderFilter](#folderfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| maxAge | MaxAge the latest object should be younger than defined age | Duration |  |
| maxCount | MinCount the minimum number of files inside the searchPath | *int |  |
| maxSize | MaxSize of the files inside the searchPath | Size |  |
| minAge | MinAge the latest object should be older than defined age | Duration |  |
| minCount | MinCount the minimum number of files inside the searchPath | *int |  |
| minSize | MinSize of the files inside the searchPath | Size |  |
| name | Name of the check | string |  |
| port | Port on which smb server is running. Defaults to 445 | int |  |
| searchPath | SearchPath sub-path inside the mount location | string |  |
| **server** | Server location of smb server. Can be `hostname/ip` or in `\\server\e$\a\b\c` syntax
Where server is the `hostname` `e$`` is the sharename and `a/b/c` is the searchPath location | string | Yes |
| sharename | Sharename to mount from the samba server | string |  |
| test |  | [Template](#template) |  |
| workstation | Workstation... | string |  |


## Template



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath |  | string |  |
| template |  | string |  |


## Connection



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | [Authentication](#authentication) |  |
| **connection** |  | string | Yes |


## AWSConnection



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **accessKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| endpoint |  | string |  |
| region |  | string |  |
| **secretKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| skipTLSVerify | Skip TLS verify when connecting to aws | bool |  |


## Bucket



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **endpoint** |  | string | Yes |
| **name** |  | string | Yes |
| **region** |  | string | Yes |


## FolderFilter



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| maxAge |  | Duration |  |
| maxSize |  | Size |  |
| minAge |  | Duration |  |
| minSize |  | Size |  |
| regex |  | string |  |


## GCPConnection



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **credentials** |  | *[kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **endpoint** |  | string | Yes |


## Authentication



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **password** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **username** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |


## Display



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |


## VarSource

VarSource represents a source for a value

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **ConfigMapKeyRef** | Selects a key of a ConfigMap. | *corev1.ConfigMapKeySelector | Yes |
| **FieldRef** | Selects a field of the pod: supports metadata.name, metadata.namespace, metadata.labels, metadata.annotations,
spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs. | *corev1.ObjectFieldSelector | Yes |
| **SecretKeyRef** | Selects a key of a secret in the pod's namespace | *corev1.SecretKeySelector | Yes |
| **Value** |  | string | Yes |


## CloudWatchFilter



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| actionPrefix |  | *string |  |
| alarmPrefix |  | *string |  |
| alarms |  | \[\]string |  |
| state |  | string |  |

