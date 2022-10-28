## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/junit.svg' style='height: 32px'/> Junit

Junit check performs a Unit test, parses the Junit test reports in a container at a specified path as defined in `testResults`.

??? example
     ```yaml
      apiVersion: canaries.flanksource.com/v1
      kind: Canary
      metadata:
        name: junit-check
        annotations:
          trace: "true"
      spec:
        interval: 120
        owner: DBAdmin
        severity: high
        spec:
          junit:
            - testResults: "/tmp/junit-results/"
              display:
                template: |
                  ✅ {{.results.passed}} ❌ {{.results.failed}} in 🕑 {{.results.duration}}
                  {{  range $r := .results.suites}}
                  {{- if gt (conv.ToInt $r.failed)  0 }}
                    {{$r.name}} ✅ {{$r.passed}} ❌ {{$r.failed}} in 🕑 {{$r.duration}}
                  {{- end }}
                  {{- end }}
              spec:
                containers:
                  - name: jes
                    image: docker.io/tarun18/junit-test-pass
                    command: ["/start.sh"]    
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| display | Template to display the result in | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **spec** | Pod specification | [v1.PodSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#podspec-v1-core) | Yes |
| test | Template to test the result against | [Template](#template) |  |
| **testResults** | Directory where the results will be published | string | Yes |
| timeout | Timeout in minutes to wait for specified container to finish its job. Defaults to 5 minutes | int |  |

---
# Scheme Reference
## Template

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath | Specify JSON path for use in template| string |  |
| template | Specify jinja template for use | string |  |
| expr | Specify expression for use in template  | string |  |
| javascript | Specify javascript syntax for template | string |  |
