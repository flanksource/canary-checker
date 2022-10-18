## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/ec2.svg' style='height: 32px'/> EC2

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: ec2-check
     spec:
       interval: 30
       spec:
         ec2:
           - description: test instance
             accessKeyID:
               valueFrom:
                 secretKeyRef:
                   name: aws-credentials
                   key: AWS_ACCESS_KEY_ID
             secretKey:
               valueFrom:
                 secretKeyRef:
                   name: aws-credentials
                   key: AWS_SECRET_ACCESS_KEY
             region: af-south-1
             userData: |
               #!/bin/bash
               yum install -y httpd
               systemctl start httpd
               systemctl enable httpd
               usermod -a -G apache ec2-user
               chown -R ec2-user:apache /var/www
               chmod 2775 /var/www
               find /var/www -type d -exec chmod 2775 {} \;
               find /var/www -type f -exec chmod 0664 {} \;
             securityGroup: WebAccess
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **accessKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| ami |  | string |  |
| canaryRef |  | \[\][v1.LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#localobjectreference-v1-core) |  |
| description | Description for the check | string |  |
| endpoint |  | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| keepAlive |  | bool |  |
| name | Name of the check | string |  |
| region |  | string |  |
| **secretKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| securityGroup |  | string |  |
| skipTLSVerify | Skip TLS verify when connecting to aws | bool |  |
| timeOut |  | int |  |
| userData |  | string |  |
| waitTime |  | int |  |
