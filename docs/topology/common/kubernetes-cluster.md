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
| schedule |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| label |  |
| owner |  |
| **components** |  |
| properties |  |
| **configs** |  |
| **lookup** |  |
| **kind** |  | string | Yes |
| labels | Labels for the check | Labels |  |
| **name** | Name of the check | string | Yes |
| namespace |  | [ResourceSelector](#resourceselector) |  |
| ready |  | *bool |  |
| resource |  | [ResourceSelector](#resourceselector) |  |
| test |  | [Template](#template) |  |
| transform |  | [Template](#template) |  |
| ignore | Ignore the specified resources from the fetched resources. Can be a glob pattern. | \[\]string |  |

