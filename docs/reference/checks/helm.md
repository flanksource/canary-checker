## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/helm.svg' style='height: 32px'/> Helm

This check builds and pushes your helm chart to the Open-source Helm Chart repository, ChartMuseum.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: helm-check
     spec:
       interval: 30
       helm:
         - chartmuseum: http://chartmuseum.default:8080
           project: library
           auth:
             username: 
               valueFrom: 
                 secretKeyRef:
                   name: helm-credentials
                   key: USERNAME
             password: 
               valueFrom: 
                 secretKeyRef:
                   name: helm-credentials
                   key: PASSWORD
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth | Username and password value, configMapKeyRef or SecretKeyRef for Helm repository | *[Authentication](#authentication) |  |
| cafile | Verify certificates of HTTPS-enabled servers in case of self-signed certificates | string |  |
| **chartmuseum** | Chartmuseum URL | string | Yes |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| project | Specify Helm project | string |  |

---
# Scheme Reference
## Authentication

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **password** | Set password for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **username** | Set username for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes | 