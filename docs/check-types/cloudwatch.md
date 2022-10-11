## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/cloudwatch.svg' style='height: 32px'/> CloudWatch

This checks the cloudwatch for all the Active alarm and response with the reason
??? example
     ```yaml
     cloudwatch:
       - accessKey:
           valueFrom:
         secretKeyRef:
         key: aws
         name: access-key
         secretKey:
           valueFrom:
         secretKeyRef:
         key: aws
         name: secrey-key
         region: "us-east-1"
         #skipTLSVerify: true
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **accessKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| endpoint |  | string |  |
| filter |  | [CloudWatchFilter](#cloudwatchfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| region |  | string |  |
| **secretKey** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| skipTLSVerify | Skip TLS verify when connecting to aws | bool |  |
| test |  | [Template](#template) |  |
