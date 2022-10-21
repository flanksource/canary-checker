## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/elasticsearch.svg' style='height: 32px'/> Elasticsearch

This check will try to connect to a specified Elasticsearch database, run a query against it and verify the results.

??? example
    ```yaml
    apiVersion: canaries.flanksource.com/v1
    kind: Canary
    metadata:
      name: elasticsearch-check
    spec:
      interval: 30
      elasticsearch:
        - url: http://elasticsearch.default.svc:9200
          description: Elasticsearch checker
          index: index
          query: |
            {
            "query": {
                "term": {
                "system.role": "api"
                }
            }
            }
            
          results: 1
          name: elasticsearch_pass
          auth:
            username: 
               valueFrom: 
                 secretKeyRef:
                   name: elasticsearch-credentials
                   key: USERNAME
            password: 
               valueFrom: 
                 secretKeyRef:
                   name: elasticsearch-credentials
                   key: PASSWORD
    ```


| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | *[Authentication](#authentication) |  |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **index** |  | string | Yes |
| labels | Labels for the check | Labels |  |
| **name** | Name of the check | string | Yes |
| **query** |  | string | Yes |
| **results** |  | int | Yes |
| test |  | [Template](#template) |  |
| transform |  | [Template](#template) |  |
| **url** |  | string | Yes |
