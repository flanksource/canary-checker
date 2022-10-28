## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/cloudwatch.svg' style='height: 32px'/> CloudWatch

This checks Cloudwatch for all the Active alarms and responses with the coresponding reasons for each. 

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: dns-pass
     spec:
       interval: 30
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
| **accessKey** | Access key value or valueFrom configMapKeyRef or SecretKeyRef to access your cloudwatch | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| description | Description for the check | string |  |
| display | Template to display the result in | [Template](#template) |  |
| endpoint | Cloudwatch HTTP Endpoint to establish connection | string |  |
| filter | Used to filter the objects | [CloudWatchFilter](#cloudwatchfilter) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| region | Region for cloudwatch | string |  |
| **secretKey** | Secret key value or valueFrom configMapKeyRef or SecretKeyRef to access cloudwatch | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| skipTLSVerify | Skip TLS verify when connecting to aws | bool |  |
| test | Template to test the result against | [Template](#template) |  |

---
# Scheme Reference
## Template

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath | Specify JSON path for use in template| string |  |
| template | Specify jinja template for use | string |  |
| expr | Specify expression for use in template  | string |  |
| javascript | Specify javascript syntax for template | string |  |


## CloudWatchFilter

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| actionPrefix | Use to filter the results of the operation to only those alarms that use a certain alarm action. For example, you could specify the ARN of an SNS topic to find all alarms that send notifications to that topic. | *string |  |
| alarmPrefix | Specify to receive information about all alarms that have names that start with this prefix. | *string |  |
| alarms | Set field to retrieve information about alarm | \[\]string |  |
| state | Specify to retrieve state value of alarm | string |  |

