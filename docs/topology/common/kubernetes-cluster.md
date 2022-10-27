## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/kubernetes.svg' style='height: 32px'/> Kubernetes

The Kubernetes topology fetches and displays a Kubernetes cluster's resources defined as `components` with types, `KubernetesNode`, and `KubernetesPod`.

??? example
    ```yaml
    apiVersion: canaries.flanksource.com/v1
    kind: SystemTemplate
    metadata:
      name: cluster
    labels:
      canary: "kubernetes-cluster"
    spec:
      type: KubernetesCluster
      icon: kubernetes
      schedule: "@every 10m"
      id:
        javascript: properties.id
      configs:
        - name: flanksource-canary-cluster
          type: EKS
      components:
        - name: nodes
          icon: server
          owner: infra
          id:
            javascript: properties.zone + "/" + self.name
          type: KubernetesNode
          lookup:
            kubernetes:
              - kind: Node
                name: k8s
                display:
                  javascript: JSON.stringify(k8s.getNodeTopology(results)) 
          properties:
            - name: node-metrics
              lookup:
                kubernetes:
                  - kind: NodeMetrics
                      ready: false
                      name: nodemetrics
                      display:
                        javascript: JSON.stringify(k8s.getNodeMetrics(results))
        - name: pods
          icon: pods
          type: KubernetesPods
          owner: Dev
          lookup:
            kubernetes:
              - kind: Pod
                name: k8s-pods
                ready: false
                ignore:
                  - junit-fail**
                  - junit-pass**
                display:
                  javascript: JSON.stringify(k8s.getPodTopology(results)) 
          properties:
            - name: pod-metrics
              lookup:
                kubernetes:
                  - kind: PodMetrics
                    ready: false
                    name: podmetrics
                    display:
                      javascript: JSON.stringify(k8s.getPodMetrics(results))  
    ```    

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| type | This specifies type of component |  | 
| id |  |
| schedule | Schedule to run checks on. Supports all cron expression, example: '30 3-6,20-23 * * *'. For more info about cron expression syntax see https://en.wikipedia.org/wiki/Cron
 Also supports golang duration, can be set as '@every 1m30s' which runs the check every 1 minute and 30 seconds. |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| owner | Owner of resource |
| **components** | Specifies structure for component to created |
| properties | Specifies properties of the component |
| **configs** | Configuration for the component |
| **lookup** |  |
| **kind** | Specifies the kind of Kubernetes object for interaction | string | Yes |
| labels | Labels for the check | Labels |  |
| **name** | Name of the check | string | Yes |
| namespace | Specifies namespce for Kubernetes object | [ResourceSelector](#resourceselector) |  |
| ready | Boolean value of true or false to query and display resources based on availability  | *bool |  |
| resource | Queries resources related to specified Kubernetes object | [ResourceSelector](#resourceselector) |  |
| test | Template to test the result against | [Template](#template) |  |
| transform | Template to transform results to | [Template](#template) |  |
| ignore | Ignore the specified resources from the fetched resources. Can be a glob pattern. | \[\]string |  |

