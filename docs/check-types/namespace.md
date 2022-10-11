## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/namespace.svg' style='height: 32px'/> Namespace

The Namespace check will:

* create a new namespace using the labels/annotations provided

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: namespace-pass
     spec:
       interval: 30
       namespace:
         - checkName: check
           namespaceNamePrefix: "test-foo-"
           podSpec: |
             apiVersion: v1
             kind: Pod
             metadata:
               name: test-namespace
               namespace: default
               labels:
                 app: hello-world-golang
             spec:
               containers:
                 - name: hello
                   image: quay.io/toni0/hello-webserver-golang:latest
           port: 8080
           path: /foo/bar
           ingressName: test-namespace-pod
           ingressHost: "test-namespace-pod.127.0.0.1.nip.io"
           readyTimeout: 5000
           httpTimeout: 15000
           deleteTimeout: 12000
           ingressTimeout: 20000
           deadline: 29000
           httpRetryInterval: 200
           expectedContent: bar
           expectedHttpStatuses: [200, 201, 202]
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **checkName** |  | string | Yes |
| deadline |  | int64 |  |
| deleteTimeout |  | int64 |  |
| description | Description for the check | string |  |
| expectedContent |  | string |  |
| expectedHttpStatuses |  | \[\]int64 |  |
| httpRetryInterval |  | int64 |  |
| httpTimeout |  | int64 |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| ingressHost |  | string |  |
| ingressName |  | string |  |
| ingressTimeout |  | int64 |  |
| name | Name of the check | string |  |
| namespaceAnnotations |  | map[string]string |  |
| namespaceLabels |  | map[string]string |  |
| namespaceNamePrefix |  | string |  |
| path |  | string |  |
| **podSpec** |  | string | Yes |
| port |  | int64 |  |
| priorityClass |  | string |  |
| readyTimeout |  | int64 |  |
| scheduleTimeout |  | int64 |  |
