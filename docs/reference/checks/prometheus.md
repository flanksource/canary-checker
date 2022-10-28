## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/prometheus.svg' style='height: 32px'/> Prometheus

The Prometheus Check connects to the Prometheus host, performs the desired query, and displays the results.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: prometheus-check
     spec:
       interval: 30
       prometheus:
         - host: http://prometheus-k8s.monitoring.svc:9090
           query: kubernetes_build_info{job!~"kube-dns|coredns"}
           display:
             template: "{{ (index .results 0).git_version }}"
           test:
             template: "true"
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| display | Template to display the result in | [Template](#template) |  |
| **host** | Address of the prometheus server | string | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **query** | PromQL query | string | Yes |
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
