# canary-checker

Kubernetes native, multi-tenant synthetic monitoring system

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://flanksource.github.io/charts | flanksource-ui | 1.0.772 |

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
| db.external.conf.db_user_namespace | string | `"off"` |  |
| db.external.conf.effective_cache_size | string | `"3GB"` |  |
| db.external.conf.effective_io_concurrency | int | `200` |  |
| db.external.conf.extra_float_digits | int | `0` |  |
| db.external.conf.log_autovacuum_min_duration | int | `0` |  |
| db.external.conf.log_connections | string | `"on"` |  |
| db.external.conf.log_destination | string | `"stderr"` |  |
| db.external.conf.log_directory | string | `"/var/log/postgresql"` |  |
| db.external.conf.log_file_mode | int | `420` |  |
| db.external.conf.log_filename | string | `"postgresql-%d.log"` |  |
| db.external.conf.log_line_prefix | string | `"%m [%p] %q[user=%u,db=%d,app=%a] "` |  |
| db.external.conf.log_lock_waits | string | `"on"` |  |
| db.external.conf.log_min_duration_statement | string | `"1s"` |  |
| db.external.conf.log_rotation_age | string | `"1d"` |  |
| db.external.conf.log_rotation_size | string | `"100MB"` |  |
| db.external.conf.log_statement | string | `"all"` |  |
| db.external.conf.log_temp_files | int | `0` |  |
| db.external.conf.log_timezone | string | `"UTC"` |  |
| db.external.conf.log_truncate_on_rotation | string | `"on"` |  |
| db.external.conf.logging_collector | string | `"on"` |  |
| db.external.conf.maintenance_work_mem | string | `"256MB"` |  |
| db.external.conf.max_connections | int | `50` |  |
| db.external.conf.max_wal_size | string | `"4GB"` |  |
| db.external.conf.password_encryption | string | `"scram-sha-256"` |  |
| db.external.conf.shared_buffers | string | `"1GB"` |  |
| db.external.conf.ssl | string | `"off"` |  |
| db.external.conf.timezone | string | `"UTC"` |  |
| db.external.conf.wal_buffers | string | `"16MB"` |  |
| db.external.conf.work_mem | string | `"10MB"` |  |
| db.external.create | bool | `false` | If false and an existing connection must be specified under secretKeyRef If create=false, a prexisting secret containing the URI to an existing postgres database must be provided   The URI must be in the format `postgresql://$user:$password@$host/$database` |
| db.external.enabled | bool | `false` | Setting to true will disable the embedded DB |
| db.external.resources.requests.memory | string | `"2Gi"` |  |
| db.external.secretKeyRef.key | string | `"DB_URL"` |  |
| db.external.secretKeyRef.name | string | `"canary-checker-postgres"` |  |
| db.external.shmVolume | string | `"256Mi"` |  |
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
| global.podAnnotations | object | `{}` |  |
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
| labelsAllowList | list | `[]` | List of additional check label keys that should be included in the check metrics |
| livenessProbe.httpGet.path | string | `"/health"` |  |
| livenessProbe.httpGet.port | int | `8080` |  |
| logLevel | string | `""` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` | node's labels for the pod to be scheduled on that node. See [Node Selector](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/) |
| otel.collector | string | `""` | OpenTelemetry gRPC collector endpoint in host:port format |
| otel.labels | string | `""` | labels in "a=b,c=d" format @schema required: false @schema |
| otel.serviceName | string | `"canary-checker"` |  |
| pingMode | string | `"unprivileged"` | set the mechanism for pings - either privileged, unprivileged or none |
| podAnnotations | object | `{}` |  |
| prometheusURL | string | `""` | Default Prometheus URL to use in prometheus checks |
| properties | object | `{}` | A map of properties to update on startup |
| readinessProbe.failureThreshold | int | `6` |  |
| readinessProbe.httpGet.path | string | `"/health"` |  |
| readinessProbe.httpGet.port | int | `8080` |  |
| readinessProbe.timeoutSeconds | int | `30` |  |
| replicas | int | `1` |  |
| resources.limits.memory | string | `"2Gi"` |  |
| resources.requests.cpu | string | `"200m"` |  |
| resources.requests.memory | string | `"200Mi"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.name | string | `"canary-checker-sa"` |  |
| serviceAccount.rbac.clusterRole | bool | `true` | whether to create cluster-wide or namespaced roles |
| serviceAccount.rbac.configmaps | bool | `true` | for secret management with valueFrom |
| serviceAccount.rbac.deploymentCreateAndDelete | bool | `true` | for deployment canary |
| serviceAccount.rbac.enabled | bool | `true` | Install (Cluster)Role and RoleBinding for the ServiceAccount |
| serviceAccount.rbac.exec | bool | `true` |  |
| serviceAccount.rbac.extra | list | `[]` |  |
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
