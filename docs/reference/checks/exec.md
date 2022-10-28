## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/exec.svg' style='height: 32px'/> Exec

Exec Check executes a command or script file on the target host.
Bash scripts are used for Linux/MacOS host. Powershell scripts are used for Windows host.

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: exec-check
     spec:
       interval: 30
       exec:
        - description: "exec dummy check"
          script: |
            echo "hello"
          name: exec-pass-check
          test:
            expr: 'results.Stdout == "hello"'
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | Description for the check | string |  |
| display | Template to display server response in text (overrides default bar format for UI) | [Template](#template) |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| labels | Labels for the check | Labels |  |
| **name** | Name of the check | string | Yes |
| **script** | Script can be a inline script or a path to a script that needs to be executed
On windows executed via powershell and in darwin and linux executed using bash | *string | Yes |
| test | Template to test the result against | [Template](#template) |  |
| transform | Template to transform results to | [Template](#template) |  |

---
# Scheme Reference
## Template

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jsonPath | Specify JSON path for use in template| string |  |
| template | Specify jinja template for use | string |  |
| expr | Specify expression for use in template  | string |  |
| javascript | Specify javascript syntax for template | string |  |
