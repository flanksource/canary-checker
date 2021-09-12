---
hide:
- toc
---

# Installation

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
