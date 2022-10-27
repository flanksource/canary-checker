## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/namespace.svg' style='height: 32px'/> Namespace

The Namespace check:

* Creates a new namespace using the labels/annotations provided
* Create a new pod in the namespace using the provided PodSpec
* Expose the pod using the provided ingress URL
* Test an HTTP connection to the pod.

??? example
     ```yaml
      apiVersion: canaries.flanksource.com/v1
      kind: Canary
      metadata:
        name: namespace-check
      spec:
        interval: 30
        spec:
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
| **checkName** | Name for the check | string | Yes |
| deadline | Overall time before which an HTTP connection to the pod must be established | int64 |  |
| deleteTimeout |  | int64 |  |
| description | Description for the check | string |  |
| expectedContent | Expected content of an HTTP response from the created pod | string |  |
| expectedHttpStatuses | Expected HTTP status code of the response from the created pod | \[\]int64 |  |
| httpRetryInterval | Interval in ms to retry HTTP connections to the created pod | int64 |  |
| httpTimeout |  | int64 |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| ingressHost | URL to be used by the ingress to expose the created pod | string |  |
| ingressName | Name to use for the ingress object that will expose the created pod | string |  |
| ingressTimeout | Maximum time to wait for an HTTP connection to the created pod | int64 |  |
| name | Name of the check | string |  |
| namespaceAnnotations | Metadata annotations to apply to created namespace | map[string]string |  |
| namespaceLabels | Metadata labels to apply to created namespace | map[string]string |  |
| namespaceNamePrefix | Prefix string to identity namespace | string |  |
| path | Path on whcih the created pod will respond to requests | string |  |
| **podSpec** | Spec of pod to be created in check namespace | string | Yes |
| port | Port on which the created pod will serve traffic | int64 |  |
| priorityClass | Pod priority class | string |  |
| readyTimeout |  | int64 |  |
| scheduleTimeout | Maximum time between pod created and pod running | int64 |  |

