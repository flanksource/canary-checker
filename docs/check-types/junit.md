## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/junit.svg' style='height: 32px'/> Junit

Junit check will wait for the given pod to be completed than parses all the xml files present in the defined testResults directory

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: junit-pass
       annotations:
         trace: "true"
     spec:
       interval: 120
       owner: DBAdmin
       severity: high
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
| display |  | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| **spec** |  | [v1.PodSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#podspec-v1-core) | Yes |
| test |  | [Template](#template) |  |
| **testResults** |  | string | Yes |
| timeout | Timeout in minutes to wait for specified container to finish its job. Defaults to 5 minutes | int |  |