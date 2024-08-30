# canary-checker

Kubernetes native, multi-tenant synthetic monitoring system

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://flanksource.github.io/charts | flanksource-ui | 1.0.751 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| allowPrivilegeEscalation | bool | `false` |  |
| canaryLabelSelector | string | `""` |  |
| canaryNamespace | string | `""` | restrict canary-checker to monitor single namespace for canaries. Leave blank to monitor all namespaces |
| canaryNamespaceSelector | string | `""` |  |
| canarySelector | string | `""` |  |
| containerdSocket | bool | `false` |  |
| db.embedded.persist | bool | `false` | persist the embedded DB with a PVC |
| db.embedded.storage | string | `"20Gi"` |  |
| db.embedded.storageClass | string | `""` |  |
| db.external.create | bool | `false` | If false and an existing connection must be specified under secretKeyRef If create=false, a prexisting secret containing the URI to an existing postgres database must be provided   The URI must be in the format `postgresql://$user:$password@$host/$database` |
| db.external.enabled | bool | `false` | Setting to true will disable the embedded DB |
| db.external.secretKeyRef.key | string | `"DB_URL"` |  |
| db.external.secretKeyRef.name | string | `"canary-checker-postgres"` |  |
| db.external.storage | string | `"20Gi"` |  |
| db.external.storageClass | string | `""` |  |
| db.runMigrations | bool | `true` |  |
| debug | bool | `false` | Turn on pprof /debug endpoint |
| disableChecks | list | `[]` | List of check types to disable |
| disablePostgrest | bool | `false` | Disable the embedded postgrest service |
| dockerSocket | bool | `false` |  |
| extra | object | `{}` |  |
| extraArgs | string | `nil` |  |
| flanksource-ui.backendURL | string | `"http://canary-checker:8080"` |  |
| flanksource-ui.enabled | bool | `true` |  |
| flanksource-ui.image.name | string | `"{{.Values.global.imagePrefix}}/canary-checker-ui"` |  |
| flanksource-ui.ingress.annotations | object | `{}` |  |
| flanksource-ui.ingress.enabled | bool | `true` |  |
| flanksource-ui.ingress.host | string | `"canary-checker-ui.local"` |  |
| flanksource-ui.ingress.tls | list | `[]` |  |
| flanksource-ui.resources.limits.cpu | string | `"200m"` |  |
| flanksource-ui.resources.limits.memory | string | `"512Mi"` |  |
| flanksource-ui.resources.requests.cpu | string | `"10m"` |  |
| flanksource-ui.resources.requests.memory | string | `"128Mi"` |  |
| global.affinity | object | `{}` |  |
| global.imagePrefix | string | `"flanksource"` |  |
| global.imageRegistry | string | `"docker.io"` |  |
| global.labels | object | `{}` |  |
| global.nodeSelector | object | `{}` | node's labels for the pod to be scheduled on that node. See [Node Selector](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/) |
| global.tolerations | list | `[]` |  |
| grafanaDashboards | bool | `false` |  |
| image.name | string | `"{{.Values.global.imagePrefix}}/canary-checker"` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.tag | string | `"latest"` |  |
| image.type | string | `"minimal"` | full image is larger and requires more permissions to run, but is required to execute 3rd party checks (jmeter, restic, k6 etc) |
| ingress.annotations | object | `{}` |  |
| ingress.className | string | `""` |  |
| ingress.enabled | bool | `false` | Expose the canary-checker service on an ingress, normally not needed as the service is exposed through `flanksource-ui.ingress` |
| ingress.host | string | `"canary-checker"` |  |
| ingress.tls | list | `[]` |  |
| jsonLogs | bool | `true` |  |
| logLevel | string | `""` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` | node's labels for the pod to be scheduled on that node. See [Node Selector](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/) |
| otel.collector | string | `""` | OpenTelemetry gRPC collector endpoint in host:port format |
| otel.labels | string | `""` | labels in "a=b,c=d" format @schema required: false @schema |
| otel.serviceName | string | `"canary-checker"` |  |
| pingMode | string | `"unprivileged"` | set the mechanism for pings - either privileged, unprivileged or none |
| prometheusURL | string | `""` | Default Prometheus URL to use in prometheus checks |
| properties | object | `{}` | A map of properties to update on startup |
| replicas | int | `1` |  |
| resources.limits.memory | string | `"2Gi"` |  |
| resources.requests.cpu | string | `"200m"` |  |
| resources.requests.memory | string | `"200Mi"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.name | string | `"canary-checker-sa"` |  |
| serviceAccount.rbac.clusterRole | bool | `true` | whether to create cluster-wide or namespaced roles |
| serviceAccount.rbac.configmaps | bool | `true` | for secret management with valueFrom |
| serviceAccount.rbac.enable | bool | `true` | Install (Cluster)Role and RoleBinding for the ServiceAccount |
| serviceAccount.rbac.exec | bool | `true` |  |
| serviceAccount.rbac.ingressCreateAndDelete | bool | `true` | for pod canary |
| serviceAccount.rbac.namespaceCreateAndDelete | bool | `true` | for namespace canary |
| serviceAccount.rbac.podsCreateAndDelete | bool | `true` | for pod and junit canaries |
| serviceAccount.rbac.readAll | bool | `true` | for use with kubernetes resource lookups |
| serviceAccount.rbac.secrets | bool | `true` | for secret management with valueFrom |
| serviceAccount.rbac.tokenRequest | bool | `true` | for secret management with valueFrom |
| serviceMonitor | bool | `false` | Set to true to enable prometheus service monitor |
| serviceMonitorLabels | object | `{}` |  |
| tolerations | list | `[]` |  |
| upstream.agentName | string | `""` |  |
| upstream.enabled | bool | `false` |  |
| upstream.host | string | `""` |  |
| upstream.insecureSkipVerify | bool | `false` |  |
| upstream.password | string | `""` |  |
| upstream.secretKeyRef | object | `{"name":null}` | Alternative to inlining values, secret must contain: AGENT_NAME, UPSTREAM_USER, UPSTREAM_PASSWORD & UPSTREAM_HOST @schema required: false @schema |
| upstream.user | string | `""` |  |
| volumeMounts | list | `[]` |  |
| volumes | list | `[]` |  |

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Flanksource |  | <https://www.flanksource.com> |
