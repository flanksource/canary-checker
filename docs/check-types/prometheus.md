## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/prometheus.svg' style='height: 32px'/> Prometheus

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
| display |  | [Template](#template) |  |
| **host** | Address of the prometheus server | string | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **query** | PromQL query | string | Yes |
| test |  | [Template](#template) |  |
