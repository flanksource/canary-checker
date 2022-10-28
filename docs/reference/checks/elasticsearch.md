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
| auth | username and password value, configMapKeyRef or SecretKeyRef for elasticsearch server | *[Authentication](#authentication) |  |
| description | Description for the check | string |  |
| display | Template to display the result in  | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **index** | Index against which query should be ran | string | Yes |
| labels | Labels for the check | Labels |  |
| **name** | Name of the check | string | Yes |
| **query** | Query that needs to be executed on the server | string | Yes |
| **results** | Number of expected hits | int | Yes |
| test | Template to test the result against | [Template](#template) |  |
| transform | Template to transform results to | [Template](#template) |  |
| **url** | host:port address | string | Yes |

---
# Scheme Reference
## Authentication

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **password** | Set password for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **username** | Set username for authentication using string, configMapKeyRef, or SecretKeyRef. | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes | 

## Template

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath | Specify JSON path for use in template| string |  |
| template | Specify jinja template for use | string |  |
| expr | Specify expression for use in template  | string |  |
| javascript | Specify javascript syntax for template | string |  |
