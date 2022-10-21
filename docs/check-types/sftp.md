## SFTPConnection

??? example
    ```yaml
    apiVersion: canaries.flanksource.com/v1
    kind: Canary
    metadata:
      name: sftp-check
    spec:
      interval: 30
      spec:
        folder:
          - path: /tmp
            name: sample sftp check
            sftpConnection:
              host: 192.168.1.5
              auth:
                username:
                  valueFrom: 
                    secretKeyRef:
                      name: sftp-credentials
                      key: USERNAME
                password:
                  valueFrom: 
                    secretKeyRef:
                      name: sftp-credentials
                      key: PASSWORD
            maxCount: 10
    ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **auth** |  | *[Authentication](#authentication) | Yes |
| **host** |  | string | Yes |
| port | Port for the SSH server. Defaults to 22 | int |  |
| maxCount | MaxCount the Maximum number of files inside the searchPath | *int |  |