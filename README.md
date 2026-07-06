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
Canary Checker is a Kubernetes-native platform for monitoring health across applications and infrastructure using both passive and active (synthetic) mechanisms.

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

| Protocol                                                     | Status     | Checks                                                       |
| ------------------------------------------------------------ | ---------- | ------------------------------------------------------------ |
| [HTTP(s)](https://canarychecker.io/reference/http)           | GA         | Response body, headers and duration                          |
| [DNS](https://canarychecker.io/reference/dns)                | GA         | Response and duration                                        |
| [Ping/ICMP](https://canarychecker.io/reference/icmp)         | GA         | Duration and packet loss                                     |
| [TCP](https://canarychecker.io/reference/tcp)                | GA         | Port is open and connectable                                 |
| **Data Sources**                                             |            |                                                              |
| [SQL](https://canarychecker.io/reference/sql) (MySQL, Postgres, SQL Server) | GA | Ability to login, results, duration, health exposed via stored procedures |
| [LDAP](https://canarychecker.io/reference/ldap)              | GA         | Ability to login, response time                              |
| [Elasticsearch](https://canarychecker.io/reference/elasticsearch) / OpenSearch | GA | Ability to login, response time, size of search results      |
| [MongoDB](https://canarychecker.io/reference/mongo)          | Beta       | Ability to login, results, duration                          |
| [Redis](https://canarychecker.io/reference/redis)            | GA         | Ability to login, results, duration                          |
| [Prometheus](https://canarychecker.io/reference/prometheus)  | GA         | Ability to login, query results and duration                  |
| **Alerts / Events**                                          |            |                                                              |
| [Prometheus Alertmanager](https://canarychecker.io/reference/alert-manager) | GA | Pending and firing alerts                                    |
| [AWS CloudWatch Alarms](https://canarychecker.io/reference/aws-cloudwatch) | GA | Pending and firing alarms                                    |
| [Dynatrace Problems](./fixtures/external/dynatrace.yaml)     | Beta       | Problems detected                                            |
| [PubSub](https://canarychecker.io/reference/pubsub)          | Beta       | Messages received from a Pub/Sub subscription                |
| [Webhook](https://canarychecker.io/reference/webhook)        | GA         | Passive checks created or updated by HTTP POST requests      |
| **DevOps**                                                   |            |                                                              |
| [Azure DevOps](https://canarychecker.io/reference/azure-devops) | Beta    | Pipeline status and duration                                 |
| Git / GitProtocol                                            | Removed    | Use `exec`, `junit` or `webhook` checks instead              |
| **Integration Testing**                                      |            |                                                              |
| [Exec](https://canarychecker.io/reference/exec)              | GA         | Run scripts or commands                                      |
| [JMeter](https://canarychecker.io/reference/jmeter)          | Beta       | Runs and checks the result of a JMeter test                  |
| [JUnit / BYO](https://canarychecker.io/reference/junit)      | Beta       | Run a pod that saves JUnit test results                      |
| [K6](https://canarychecker.io/examples/k6)                   | Beta       | Runs k6 tests that export JUnit via a container              |
| [Newman](https://canarychecker.io/examples/newman)           | Beta       | Runs Newman / Postman tests that export JUnit via a container |
| [Playwright](https://canarychecker.io/examples/Playwright)   | Beta       | Runs Playwright tests that export JUnit via a container      |
| **File Systems / Batch**                                     |            |                                                              |
| [Local Disk / NFS](https://canarychecker.io/reference/folder) | GA        | Check folders for files that are too few/many, too old/new, too small/large |
| [S3](https://canarychecker.io/reference/folder#s3)           | GA         | Check contents of AWS S3 buckets                             |
| [GCS](https://canarychecker.io/reference/folder#gcs)         | GA         | Check contents of Google Cloud Storage buckets               |
| [SFTP](https://canarychecker.io/reference/folder#sftp)       | GA         | Check contents of folders over SFTP                          |
| [SMB / CIFS](https://canarychecker.io/reference/folder#smb)  | GA         | Check contents of folders over SMB/CIFS                      |
| **Config**                                                   |            |                                                              |
| [AWS Config](https://canarychecker.io/reference/aws-config)  | GA         | Query AWS Config using SQL                                   |
| [AWS Config Rule](https://canarychecker.io/reference/aws-config-rule) | GA  | AWS Config rules that are firing, custom AWS Config queries  |
| [Config DB / Catalog](https://canarychecker.io/reference/catalog) | GA    | Custom config queries for Mission Control Config DB          |
| [Kubernetes](https://canarychecker.io/reference/kubernetes)  | GA         | Kubernetes resources that are missing or in a non-ready state |
| [Kubernetes Resource](https://canarychecker.io/reference/kubernetes-resource) | GA | Create temporary resources, run nested checks and clean up   |
| **Backups**                                                  |            |                                                              |
| [GCP Databases](https://canarychecker.io/reference/gcs-database-backup) | GA | Backup freshness                                             |
| [Restic](https://canarychecker.io/reference/restic)          | Beta       | Backup freshness and integrity                               |
| **Infrastructure**                                           |            |                                                              |
| [S3 Protocol](https://canarychecker.io/reference/s3-protocol) | GA        | Ability to read/write/list objects on an S3 compatible object store |
| Pod / Namespace                                              | Removed    | Use `kubernetesResource` or `kubernetes` checks instead      |
| Docker / Containerd                                          | Removed    | Use `kubernetesResource` or `exec` checks instead            |
| Helm                                                         | Removed    | Use `kubernetesResource` or `exec` checks instead            |

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md)

Thank you to all our contributors!

<a href="https://github.com/flanksource/canary-checker/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=flanksource/canary-checker" />
</a>

## License

Canary Checker core (the code in this repository) is licensed under [Apache 2.0](./LICENSE) and accepts contributions via GitHub pull requests after signing a CLA.

The UI (Dashboard) is free to use with Canary Checker under a license exception of [Flanksource UI](https://github.com/flanksource/flanksource-ui/blob/main/LICENSE#L7)
