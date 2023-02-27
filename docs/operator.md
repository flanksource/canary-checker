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
You only need to define things that are different from the defaults listed in
that file. After configuring the values.yaml file, install Canary Checker with

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
