<div align="center">
  <picture>
    <source srcset="https://canarychecker.io/img/canary-checker-white.svg" media="(prefers-color-scheme: dark)">
    <img src="https://canarychecker.io/img/canary-checker.svg">
  </picture>

  <p>Kubernetes Native Health Check Platform</p>
  <p>
    <a href="https://github.com/flanksource/canary-checker/actions/workflows/test.yml"><img src="https://github.com/flanksource/canary-checker/actions/workflows/test.yml/badge.svg?branch=master"></a>
    <a href="https://github.com/flanksource/canary-checker/actions/workflows/lint.yml"><img src="https://github.com/flanksource/canary-checker/actions/workflows/lint.yml/badge.svg?branch=master"></a>
    <a href="https://github.com/flanksource/canary-checker/actions/workflows/codeql.yml"><img src="https://github.com/flanksource/canary-checker/actions/workflows/codeql.yml/badge.svg?branch=master"></a>
    <a href="https://hub.docker.com/r/flanksource/canary-checker"><img src="https://img.shields.io/docker/pulls/flanksource/canary-checker?logo=docker&style=flat-square"></a>
    <a href="https://securityscorecards.dev/viewer/?uri=github.com/flanksource/canary-checker"><img src="https://api.securityscorecards.dev/projects/github.com/flanksource/canary-checker/badge"></a>
    <img src="https://img.shields.io/github/license/flanksource/canary-checker.svg?style=flat-square"/>
    <a href="https://canarychecker.io"> <img src="https://img.shields.io/badge/☰-Docs-lightgrey.svg"/></a>
  </p>
</div>

---
Canary Checker is a Kubernetes-native platform for monitoring application and infrastructure health using both active synthetic checks and passive signal ingestion.

## Features

* **Batteries Included** - 30+ built-in active and passive check types
* **Kubernetes Native** - Health checks, or canaries, are CRDs that reflect health via the `status` field, making them compatible with GitOps, [Flux Health Checks](https://fluxcd.io/flux/components/kustomize/kustomization/#health-checks), Argo, Helm, etc.
* **Secret Management** - Leverage Kubernetes Secrets and ConfigMaps for authentication and connection details
* **Prometheus** - Prometheus-compatible metrics are exposed at `/metrics`. A Grafana dashboard is also available.
* **Self-Contained Storage** - Uses embedded Postgres by default and can be configured to use external Postgres.
* **JUnit Export (CI/CD)**  - Export health check results to JUnit format for integration into CI/CD pipelines
* **JUnit Import (k6/Newman/Puppeteer/etc.)** - Use any container that creates JUnit test results
* **Scriptable** - Go templates, JavaScript and [CEL](https://canarychecker.io/scripting/cel) can be used to:
  * Evaluate whether a check is passing and severity to use when failing
  * Extract a user friendly error message
  * Transform and filter check responses into individual check results
  * Extract custom metrics
* **Multi-Modal** - While designed as a Kubernetes operator, Canary Checker can also run as a CLI and as a standalone server.

## Getting Started

1. Install Canary Checker with Helm

```shell
helm repo add flanksource https://flanksource.github.io/charts
helm repo update

helm install \
  canary-checker \
  flanksource/canary-checker \
  -n canary-checker \
  --create-namespace \
  --wait
```

2. Create a new check

```yaml title="canary.yaml"
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: http-check
spec:
  schedule: "@every 30s"
  http:
    - name: basic-check
      url: https://httpbin.flanksource.com/status/200
    - name: failing-check
      url: https://httpbin.flanksource.com/status/500
```

2a. Run the check locally (Optional)

```shell
wget  https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker_linux_amd64 \
-O canary-checker &&  chmod +x canary-checker
./canary-checker run canary.yaml
```

[![asciicast](https://asciinema.org/a/cYS6hlmX516JQeECHH7za3IDG.svg)](https://asciinema.org/a/cYS6hlmX516JQeECHH7za3IDG)

3. Apply the check

```shell
kubectl apply -f canary.yaml
```

4. Check the health status

```shell
kubectl get canary
```

``` title="sample output"
NAME         SCHEDULE      STATUS   LAST CHECK   UPTIME 1H        LATENCY 1H   LAST TRANSITIONED
http-check   @every 30s    Passed   13s          18/18 (100.0%)   480ms        13s
```

See [fixtures](https://github.com/flanksource/canary-checker/tree/master/fixtures) for more examples and [docs](https://canarychecker.io/getting-started) for more comprehensive documentation.

## Use Cases

### Synthetic Testing

Run simple HTTP/DNS/ICMP probes or more advanced full test suites using JMeter, k6, Playwright, Postman/Newman, or any container that can emit JUnit XML.

```yaml
# Run a container that executes a playwright test, and then collect the
# JUnit formatted test results from the /tmp folder
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: playwright-junit
spec:
  schedule: "@every 2m"
  junit:
    - testResults: "/tmp/"
      name: playwright-junit
      spec:
        containers:
          - name: playwright
            image: ghcr.io/flanksource/canary-playwright:latest
```

### Infrastructure Testing

Verify that infrastructure is fully operational by checking Kubernetes resources, deploying temporary resources, and running nested checks against them.

```yaml
# Deploy a temporary pod and service, wait for readiness, call the service, and then clean up.
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: kubernetes-resource-check
spec:
  schedule: "@every 5m"
  kubernetesResource:
    - name: service-accessibility
      namespace: default
      waitFor:
        expr: 'dyn(resources).all(r, k8s.isReady(r))'
        interval: 2s
        timeout: 2m
      resources:
        - apiVersion: v1
          kind: Pod
          metadata:
            name: httpbin-pod
            namespace: default
            labels:
              app: httpbin
          spec:
            containers:
              - name: httpbin
                image: kennethreitz/httpbin:latest
                ports:
                  - containerPort: 80
        - apiVersion: v1
          kind: Service
          metadata:
            name: httpbin
            namespace: default
          spec:
            selector:
              app: httpbin
            ports:
              - port: 80
                targetPort: 80
      checks:
        - http:
            - name: call-httpbin-service
              url: http://httpbin.default.svc/status/200
      checkRetries:
        delay: 2s
        interval: 3s
        timeout: 2m
```

### Backup Checks / Batch File Monitoring

Check that batch file processes are functioning correctly by checking the age and size of files in local file systems, SFTP, SMB, S3 and GCS.

```yaml
# Checks that a recent DB backup has been uploaded
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: folder-check
spec:
  schedule: "0 22 * * *"
  folder:
    - path: s3://database-backups/prod
      name: prod-backup
      maxAge: 1d
      minSize: 10gb
```

### Alert Aggregation

Aggregate alerts and recommendations from Prometheus, AWS CloudWatch, Dynatrace, etc.

```yaml
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: alertmanager-check
spec:
  schedule: "*/5 * * * *"
  alertmanager:
    - url: alertmanager.monitoring.svc
      alerts:
        - .*
      ignore:
        - KubeScheduler.*
        - Watchdog
      transform:
        # for each alert, transform it into a new check
        javascript: |
          var out = _.map(results, function(r) {
            return {
              name: r.name,
              labels: r.labels,
              icon: 'alert',
              message: r.message,
              description: r.message,
            }
          })
          JSON.stringify(out);
```

### Prometheus Exporter Replacement

Export [custom metrics](https://canarychecker.io/concepts/metrics-exporter) from the result of any check, making it possible to replace various other Prometheus exporters that collect metrics via HTTP, SQL, etc.

```yaml
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: exchange-rates
spec:
  schedule: "@every 1h"
  http:
    - name: exchange-rates
      url: https://api.frankfurter.app/latest?from=USD&to=GBP,EUR,ILS
      metrics:
        - name: exchange_rate
          type: gauge
          value: result.json.rates.GBP
          labels:
            - name: "from"
              value: "USD"
            - name: to
              value: GBP
```

## Platform Ready

Canary Checker is ideal for building platforms. Developers can include health checks for their applications in whatever tooling they prefer, with secret management that uses native Kubernetes constructs.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: basic-auth
stringData:
  user: john
  pass: doe
---
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: http-basic-auth-secret
spec:
  http:
    - name: http-basic-auth
      url: https://httpbin.flanksource.com/basic-auth/john/doe
      username:
        valueFrom:
          secretKeyRef:
            name: basic-auth
            key: user
      password:
        valueFrom:
          secretKeyRef:
            name: basic-auth
            key: pass
```

## Dashboard

Canary Checker comes with a built-in dashboard by default.

![](https://canarychecker.io/img/canary-ui.png)

There is also a [Grafana](https://canarychecker.io/concepts/grafana) dashboard, or build your own using the metrics exposed.

## Getting Help

If you have any questions about Canary Checker:

* Read the [docs](https://canarychecker.io)
* Invite yourself to the [CNCF community slack](https://slack.cncf.io/) and join the [#canary-checker](https://cloud-native.slack.com/messages/canary-checker/) channel.
* Check out the [YouTube playlist](https://www.youtube.com/playlist?list=PLz4F_KggvA58D6krlw433TNr8qMbu1aIU).
* File an [issue](https://github.com/flanksource/canary-checker/issues/new) - (We do provide user support via GitHub Issues, so don't worry if your issue is a bug or not)

Your feedback is always welcome!

## Check Types

This is the current high-level inventory. See the [reference docs](https://canarychecker.io/reference) for full schemas and examples.

| Category | Current checks |
| --- | --- |
| Network | [HTTP(s)](https://canarychecker.io/reference/http), [DNS](https://canarychecker.io/reference/dns), [ICMP](https://canarychecker.io/reference/icmp), [TCP](https://canarychecker.io/reference/tcp) |
| Data sources | [Postgres, MySQL and SQL Server](https://canarychecker.io/reference/sql), [LDAP](https://canarychecker.io/reference/ldap), [MongoDB](https://canarychecker.io/reference/mongo), [Redis](https://canarychecker.io/reference/redis), [Prometheus](https://canarychecker.io/reference/prometheus), [Elasticsearch](https://canarychecker.io/reference/elasticsearch), OpenSearch |
| Alerts and integrations | [Alertmanager](https://canarychecker.io/reference/alert-manager), [AWS CloudWatch](https://canarychecker.io/reference/aws-cloudwatch), Dynatrace, [Azure DevOps](https://canarychecker.io/reference/azure-devops), [PubSub](https://canarychecker.io/reference/pubsub), [webhooks](https://canarychecker.io/reference/webhook) |
| Configuration and cloud | [AWS Config](https://canarychecker.io/reference/aws-config), [AWS Config Rule](https://canarychecker.io/reference/aws-config-rule), [Config DB / catalog](https://canarychecker.io/reference/catalog), [Kubernetes](https://canarychecker.io/reference/kubernetes), [Kubernetes Resource](https://canarychecker.io/reference/kubernetes-resource) |
| Files and backups | [Folder](https://canarychecker.io/reference/folder) checks for local/NFS, S3, GCS, SFTP and SMB/CIFS; [S3 protocol](https://canarychecker.io/reference/s3-protocol); [GCP database backups](https://canarychecker.io/reference/gcs-database-backup); [Restic](https://canarychecker.io/reference/restic) |
| Test runners and custom checks | [Exec](https://canarychecker.io/reference/exec), [JMeter](https://canarychecker.io/reference/jmeter), [JUnit / BYO](https://canarychecker.io/reference/junit), plus k6, Newman/Postman and Playwright via JUnit-producing containers |

Legacy compatibility fields still exist in the CRD for `pod`, `namespace`, `docker`, `dockerPush`, `containerd`, `containerdPush`, `helm`, `github` and `gitProtocol`, but the runner reports them as removed. Use `kubernetesResource`, `kubernetes`, `exec`, `junit` or `webhook` checks for those workflows.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md)

Thank you to all our contributors!

<a href="https://github.com/flanksource/canary-checker/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=flanksource/canary-checker" />
</a>

## License

Canary Checker core (the code in this repository) is licensed under [Apache 2.0](./LICENSE) and accepts contributions via GitHub pull requests after signing a CLA.

The UI (Dashboard) is free to use with Canary Checker under a license exception of [Flanksource UI](https://github.com/flanksource/flanksource-ui/blob/main/LICENSE#L7)
