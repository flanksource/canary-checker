## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/pod.svg' style='height: 32px'/> Pod

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: pod-pass
     spec:
       interval: 30
       pod:
         - name: golang
           namespace: default
           spec: |
             apiVersion: v1
             kind: Pod
             metadata:
               name: hello-world-golang
               namespace: default
               labels:
                 app: hello-world-golang
             spec:
               containers:
                 - name: hello
                   image: quay.io/toni0/hello-webserver-golang:latest
           port: 8080
           path: /foo/bar
           ingressName: hello-world-golang
           ingressHost: "hello-world-golang.127.0.0.1.nip.io"
           scheduleTimeout: 20000
           readyTimeout: 10000
           httpTimeout: 7000
           deleteTimeout: 12000
           ingressTimeout: 10000
           deadline: 60000
           httpRetryInterval: 200
           expectedContent: bar
           expectedHttpStatuses: [200, 201, 202]
           priorityClass: canary-checker-priority
         - name: ruby
           namespace: default
           spec: |
             apiVersion: v1
             kind: Pod
             metadata:
               name: hello-world-ruby
               namespace: default
               labels:
                 app: hello-world-ruby
             spec:
               containers:
                 - name: hello
                   image: quay.io/toni0/hello-webserver-ruby:latest
                   imagePullPolicy: Always
           port: 8080
           path: /foo/bar
           ingressName: hello-world-ruby
           ingressHost: "hello-world-ruby.127.0.0.1.nip.io"
           scheduleTimeout: 30000
           readyTimeout: 12000
           httpTimeout: 7000
           deleteTimeout: 12000
           ingressTimeout: 10000
           deadline: 29000
           httpRetryInterval: 200
           expectedContent: hello, you've hit /foo/bar
           expectedHttpStatuses: [200, 201, 202]
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| deadline |  | int64 |  |
| deleteTimeout |  | int64 |  |
| description | Description for the check | string |  |
| expectedContent |  | string |  |
| expectedHttpStatuses |  | \[\]int |  |
| httpRetryInterval |  | int64 |  |
| httpTimeout |  | int64 |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| **ingressHost** |  | string | Yes |
| **ingressName** |  | string | Yes |
| ingressTimeout |  | int64 |  |
| name | Name of the check | string |  |
| **namespace** |  | string | Yes |
| path |  | string |  |
| port |  | int64 |  |
| priorityClass |  | string |  |
| readyTimeout |  | int64 |  |
| scheduleTimeout |  | int64 |  |
| **spec** |  | string | Yes |
