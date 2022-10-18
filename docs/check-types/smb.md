## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/smb.svg' style='height: 32px'/> Smb

Smb check will connect to the given samba server with given credentials
find the age of the latest updated file and compare it with minAge
count the number of file present and compare with minCount if defined

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: sftp-check
     spec:
       interval: 30
       folder:
         - path: /tmp
           name: sample smb check
           smbConnection:
             server: \\server\e$
             auth:
               username:
                 valueFrom: 
                   secretKeyRef:
                     name: smb-credentials
                     key: USERNAME
               password:
                 valueFrom: 
                   secretKeyRef:
                     name: smb-credentials
                     key: PASSWORD
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
| maxCount | MaxCount the Maximum number of files inside the searchPath | *int |  |
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


