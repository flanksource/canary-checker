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
| auth |  | *[Authentication](#authentication) |  |
| cafile |  | string |  |
| **chartmuseum** |  | string | Yes |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| project |  | string |  |