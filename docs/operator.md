---
hide:
- toc
---

# Installation

## Helm

Canary Checker can be deployed to a Kubernetes cluster via Helm.

```bash
helm repo add flanksource https://flanksource.github.io/charts
helm repo update flanksource
```

Create a values file based on
[values.yaml](https://github.com/flanksource/canary-checker/blob/master/chart/values.yaml).
You only need to define things that are different from the defaults:

```yaml
# Default values for canary-checker.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

replicas: 1

image:
  repository: docker.io/flanksource/canary-checker
  pullPolicy: IfNotPresent
  # Overrides the image tag whose default is the chart appVersion.
  tag: "latest"

dockerSocket: true
containerdSocket: false

# Set to true if you want to disable the postgrest service
disablePostgrest: false

# Turn on pprof /debug endpoint
debug: false
logLevel: "-v"

db:
  embedded:
    # If the database is embedded, setting this to true will persist the contents of the database
    # through a persistent volume
    persist: true
    storageClass:
    storage:
  external:
    # Setting enabled to true will use a external postgres DB, disabling the embedded DB
    enabled: false
    # Setting create to true will create a postgres stateful set for config-db to connect to.
    # If create=true, the secretKeyRef will be created by helm with the specified name and key
    #   Optionally populate a secret named 'postgres-connection' before install with POSTGRES_USER and POSTGRES_PASSWORD to set the created username and password, otherwise a random password will be created for a 'postgres' user
    # If create=false, a prexisting secret containing the URI to an existing postgres database must be provided
    #   The URI must be in the format 'postgresql://"$user":"$password"@"$host"/"$database"'
      # Setting this to true will provision a new postgress DB for you
    create: false
    secretKeyRef:
      name: canary-checker-postgres
      # This is the key that either the secret will create(if create is true) or
      # this is the key it will look for in the secret(if secretRefKey is
      # mentioned). The name of the key is mandatory to set.
      key: DB_URL
    storageClass:
    storage:

nameOverride: ""

ingress:
  enabled: false
  className: ""
  annotations:
    {}
    # kubernetes.io/ingress.class: nginx
    # kubernetes.io/tls-acme: "true"
  host: canary-checker
  tls: []
  #  - secretName: chart-example-tls
  #    hosts:
  #      - chart-example.local

flanksource-ui:
  enabled: true
  nameOverride: "canary-checker-ui"
  fullnameOverride: "canary-checker-ui"
  oryKratosURL: ""
  # Mandatory.  Set to the name of the service installed by the chart (RFC1035 formatted $RELEASE_NAME)
  backendURL: "canary-checker"
  ingress:
    enabled: true
    host: "canary-checker-ui.local"
    annotations:
      {}
      # kubernetes.io/ingress.class: nginx
      # kubernetes.io/tls-acme: "true"
    tls: []
    #  - secretName: chart-example-tls
    #    hosts:
    #      - chart-example.local

resources:
  requests:
    cpu: 200m
    memory: 200Mi
  limits:
    memory: 1512Mi

extra:
  # nodeSelector:
  #   key: value
  # tolerations:
  #   - key: "key1"
  #     operator: "Equal"
  #     value: "value1"
  #     effect: "NoSchedule"
  # affinity:
  #   nodeAffinity:
  #       requiredDuringSchedulingIgnoredDuringExecution:
  #         nodeSelectorTerms:
  #         - matchExpressions:
  #           - key: kubernetes.io/e2e-az-name
  #             operator: In
  #             values:
  #             - e2e-az1
  #             - e2e-az2
```

After configuring the values.yaml file, install Canary Checker with


```bash
helm install --namespace <NAMESPACE> canary-checker -f <VALUES_FILE> flanksource/canary-checker
```

## Manually

=== "kubectl"
    ```bash
    kubectl apply -f https://github.com/flanksource/canary-checker/releases/latest/download/release.yaml
    ```

=== "kustomize"
    `kustomization.yaml`
    ```yaml
    resources:
    - https://raw.githubusercontent.com/flanksource/canary-checker/v0.38.30/config/deploy/base.yaml
    - https://raw.githubusercontent.com/flanksource/canary-checker/v0.38.30/config/deploy/crd.yaml
    images:
    - name: docker.io/flanksource/canary-checker:latest
      newTag: v0.38.30
    patchesStrategicMerge:
    - |-
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        metadata:
            name: canary-checker
            namespace: canary-checker
        spec:
            tls:
            - hosts:
                - <INSERT_YOUR_DOMAIN_HERE>
        rules:
            - host:  <INSERT_YOUR_DOMAIN_HERE>
    ```

    ```bash
    kubectl apply -f kustomization.yaml
    ```

## Karina

When deploying Canary Checker via
[Karina](https://karina.docs.flanksource.com/), the versions of
Canary Checker and Flanksource UI are specified in `karina.yml`. The configured
persistence will define how Canary Checker stores its database.

```yaml
canaryChecker:
  version: v0.38.194
  uiVersion: v1.0.201
  persistence:
    capacity: 20Gi
    storageClass: standard
```

## Installing without a database

Canary Checker ships with a database by default, which keeps a history of
checks. If Canary Checker doesn't require persistence, it can be disabled by
setting `db.embedded.persist` to `false` in the values configuration file.

In Karina-based deployments, the database can be disabled by setting
`canaryChecker.persistence.disabled` to `true` in `karina.yml` (for Karina
versions >= v0.70.0). For versions older than this, the following patch will
achieve the same result:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: canary-checker
  namespace: platform-system
spec:
  template:
    spec:
      volumes:
        - name: canarychecker-database
          emptyDir: {}
  volumeClaimTemplates: null
```

# Running Manually

The operator can also be started manually outside of cluster (it will still need a kubeconfig) using:

```bash
canary-checker operator [flags]
```

### Options

```
      --dev                       Run in development mode
      --devGuiPort int            Port used by a local npm server in development mode (default 3004)
      --enable-leader-election    Enabling this will ensure there is only one active controller manager
  -h, --help                      help for operator
      --httpPort int              Port to expose a health dashboard  (default 8080)
      --include-check string      Run matching canaries - useful for debugging
      --log-fail                  Log every failing check (default true)
      --log-pass                  Log every passing check
      --maxStatusCheckCount int   Maximum number of past checks in the status page (default 5)
      --metricsPort int           Port to expose a health dashboard  (default 8081)
      --name string               Server name shown in aggregate dashboard (default "local")
  -n, --namespace string          Watch only specified namespaces, otherwise watch all
      --prometheus string         URL of the prometheus server that is scraping this instance
      --pull-servers strings      push check results to multiple canary servers
      --push-servers strings      push check results to multiple canary servers
      --webhookPort int           Port for webhooks  (default 8082)
      --expose-env                Expose environment variables for use in all templates. Note this has serious security implications with untrusted canaries
      --json-logs                 Print logs in json format to stderr
  -v, --loglevel count            Increase logging level
```
