<div align="center">
  <picture>
    <source srcset="https://canarychecker.io/img/canary-checker-white.svg" media="(prefers-color-scheme: dark)">
    <img src="https://canarychecker.io/img/canary-checker.svg">
  </picture>

  <p>Kubernetes Native Health Check Platform</p>
  <p>
    <a href="https://github.com/flanksource/canary-checker/actions"><img src="https://github.com/flanksource/canary-checker/workflows/Test/badge.svg"></a>
    <a href="https://goreportcard.com/report/github.com/flanksource/canary-checker"><img src="https://goreportcard.com/badge/github.com/flanksource/canary-checker"></a>
    <img src="https://img.shields.io/github/license/flanksource/canary-checker.svg?style=flat-square"/>
    <a href="https://canarychecker.io"> <img src="https://img.shields.io/badge/☰-Docs-lightgrey.svg"/></a>
  </p>
</div>

---
Canary checker is a kubernetes-native platform for monitoring health across application and infrastructure using both passive and active (synthetic) mechanisms.

## Features

* **Batteries Included** - 35+ built-in check types
* **Kubernetes Native** - Health checks (or canaries) are CRD's that reflect health via the `status` field, making them compatible with GitOps, [Flux Health Checks](https://fluxcd.io/flux/components/kustomize/kustomization/#health-checks), Argo, Helm, etc..
* **Secret Management** - Leverage K8S secrets and configmaps for authentication and connection details
* **Prometheus** - Prometheus compatible metrics are exposed at `/metrics`.  A Grafana Dashboard is also available.
* **Dependency Free** - Runs an embedded postgres instance by default,  can also be configured to use an external database.
* **JUnit Export (CI/CD)**  - Export health check results to JUnit format for integration into CI/CD pipelines
* **JUnit Import (k6/newman/puppeter/etc)** - Use any container that creates JUnit test results
* **Scriptable** - Go templates, Javascript and [CEL](https://canarychecker.io/scripting/cel) can be used to:
  * Evaluate whether a check is passing and severity to use when failing
  * Extract a user friendly error message
  * Transform and filter check responses into individual check results
  * Extract custom metrics
* **Multi-Modal** - While designed as a Kubernetes Operator, canary checker can also run as a CLI and a server without K8s

## Getting Started

1. Install canary checker with Helm

```shell
helm repo add flanksource https://flanksource.github.io/charts
helm repo update

helm install \
  canary-checker \
  flanksource/canary-checker \
 -n canary-checker \
 --create-namespace
 --wait
  ```

2. Create a new check

  ```yaml title="canary.yaml"
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: http-check
spec:
  interval: 30
  http:
    - name: basic-check
      url: https://httpbin.demo.aws.flanksource.com/status/200
    - name: failing-check
      url: https://httpbin.demo.aws.flanksource.com/status/500
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
NAME               INTERVAL   STATUS   LAST CHECK   UPTIME 1H        LATENCY 1H   LAST TRANSITIONED
http-check.        30         Passed   13s          18/18 (100.0%)   480ms        13s
```

See [fixtures](https://github.com/flanksource/canary-checker/tree/master/fixtures) for more examples and [docs](https://canarychecker.io/getting-started) for more comprehensive documentation.

## Use Cases

### Synthetic Testing

Run simple HTTP/DNS/ICMP probes or more advanced full test suites using JMeter, K6, Playright, Postman.

```yaml
# Run a container that executes a playwright test, and then collect the
# JUnit formatted test results from the /tmp folder
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: playwright-junit
spec:
  interval: 120
  junit:
    - testResults: "/tmp/"
      name: playwright-junit
      spec:
        containers:
          - name: playwright
            image: ghcr.io/flanksource/canary-playwright:latest
```

### Infrastructure Testing

Verify that infrastructure is fully operational by [deploying new pods](https://canarychecker.io/reference/pod), spinning up new EC2 instances and pushing/pulling from docker and helm repositories.

```yaml
# Schedule a new pod with an ingress and then time how long it takes to schedule, be ready, respond to an http request and finally be cleaned up.
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: pod-check
spec:
  interval: 30
  pod:
    - name: golang
      spec: |
        apiVersion: v1
        kind: Pod
        metadata:
          name: hello-world-golang
          namespace: default
          labels:
            app: hello-world-golang
        spec:
          containers:
            - name: hello
              image: quay.io/toni0/hello-webserver-golang:latest
      port: 8080
      path: /foo/bar
      scheduleTimeout: 20000
      readyTimeout: 10000
      httpTimeout: 7000
      deleteTimeout: 12000
      ingressTimeout: 10000
      deadline: 60000
      httpRetryInterval: 200
      expectedContent: bar
      expectedHttpStatuses: [200, 201, 202]
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
  schedule: 0 22 * * *
  folder:
    - path: s3://database-backups/prod
      name: prod-backup
      maxAge: 1d
      minSize: 10gb
```

### Alert Aggregation

Aggregate alerts and recommendations from Prometheus, AWS Cloudwatch, Dynatrace, etc.

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

Export [custom metrics](https://canarychecker.io/concepts/metrics-exporter) from the result of any check, making it possible to replace various other promethus exporters that collect metrics via HTTP, SQL, etc..

```yaml
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: exchange-rates
spec:
  schedule: "every 1 @hour"
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

Canary checker is ideal for building platforms, developers can include health checks for their applications in whatever tooling they prefer, with secret management that uses native Kubernetes constructs.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name:  basic-auth
stringData:
   user: john
   pass: doe
---
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: http-basic-auth-configmap
spec:
  http:
    - url: https://httpbin.demo.aws.flanksource.com/basic-auth/john/doe
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

Canary checker comes with a built-in dashboard by default

![](https://canarychecker.io/img/canary-ui.png)

There is also a [grafana](https://canarychecker.io/concepts/grafana) dashboard, or build your own using the metrics exposed.

## Getting Help

If you have any questions about canary checker:

* Read the [docs](https://canarychecker.io)
* Invite yourself to the [CNCF community slack](https://slack.cncf.io/) and join the [#canary-checker](https://cloud-native.slack.com/messages/canary-checker/) channel.
* Check out the [Youtube Playlist](https://www.youtube.com/playlist?list=PLz4F_KggvA58D6krlw433TNr8qMbu1aIU).
* File an [issue](https://github.com/flanksource/canary-checker/issues/new) - (We do provide user support via Github Issues, so don't worry if your issue is a bug or not)

Your feedback is always welcome!

## Check Types

| Protocol                                                     | Status     | Checks                                                       |
| ------------------------------------------------------------ | ---------- | ------------------------------------------------------------ |
| [HTTP(s)](https://canarychecker.io/reference/http)                                 | GA         | Response body, headers and duration                          |
| [DNS](https://canarychecker.io/reference/dns)                                      | GA         | Response and duration                                        |
| [Ping/ICMP](https://canarychecker.io/reference/icmp)                               | GA         | Duration and packet loss                                     |
| [TCP](https://canarychecker.io/reference/tcp)                                      | GA         | Port is open and connectable                                 |
| **Data Sources**                                             |            |                                                              |
| SQL ([MySQL](https://canarychecker.io/reference/mysql), [Postgres](https://canarychecker.io/reference/postgres), [SQL Server](https://canarychecker.io/reference/mssql)) | GA         | Ability to login, results, duration, health exposed via stored procedures |
| [LDAP](https://canarychecker.io/reference/ldap)                                    | GA         | Ability to login, response time                              |
| [ElasticSearch / Opensearch](https://canarychecker.io/reference/elasticsearch)     | GA         | Ability to login, response time, size of search results      |
| [Mongo](https://canarychecker.io/reference/mongo)                                  | Beta       | Ability to login, results, duration,                         |
| [Redis](https://canarychecker.io/reference/redis)                                  | GA         | Ability to login, results, duration,                         |
| [Prometheus](https://canarychecker.io/reference/prometheus)                        | GA         | Ability to login, results, duration,                         |
| **Alerts**                                                   |            | Prometheus                                                   |
| [Prometheus Alert Manager](https://canarychecker.io/reference/alert-manager)       | GA         | Pending and firing alerts                                    |
| [AWS Cloudwatch Alarms](https://canarychecker.io/reference/cloudwatch)             | GA         | Pending and firing alarms                                    |
| [Dynatrace Problems](./reference/dynatrace.md)               | Beta       | Problems deteced                                             |
| **DevOps**                                                   |            |                                                              |
| [Git](https://canarychecker.io/reference/git)                                      | GA         | Query Git and Github repositories via SQL                    |
| [Azure Devops](https://canarychecker.io/reference)                                 | Beta |                                                              |
| **Integration Testing**                                      |            |                                                              |
| [JMeter](https://canarychecker.io/reference/jmeter)                                | Beta       | Runs and checks the result of a JMeter test                  |
| [JUnit / BYO](https://canarychecker.io/reference/junit)                            | Beta       | Run a pod that saves Junit test results                      |
| [K6](https://canarychecker.io/reference/k6) | Beta | Runs K6 tests that export JUnit via a container |
| [Newman](https://canarychecker.io/reference/newman) | Beta |  Runs Newman / Postman tests that export JUnit via a container  |
| [Playwright](https://canarychecker.io/reference/Playwright) | Beta |  Runs Playwright tests that export JUnit via a container  |
| **File Systems / Batch**                                     |            |                                                              |
| [Local Disk / NFS](https://canarychecker.io/reference/folder)                      | GA         | Check folders for files that are:  too few/many, too old/new, too small/large |
| [S3](https://canarychecker.io/reference/s3-bucket)                                 | GA         | Check contents of AWS S3 Buckets                             |
| [GCS](https://canarychecker.io/reference/gcs-bucket)                               | GA         | Check contents of Google Cloud Storage Buckets               |
| [SFTP](https://canarychecker.io/reference/sftp)                                    | GA         | Check contents of folders over SFTP                          |
| [SMB / CIFS](../smb)                                         | GA         | Check contents of folders over SMB/CIFS                      |
| **Config**                                                   |            |                                                              |
| [AWS Config](https://canarychecker.io/reference/aws-config)                        | GA         | Query AWS config using SQL                                   |
| [AWS Config Rule](https://canarychecker.io/reference/aws-config-rule)              | GA         | AWS Config Rules that are firing, Custom AWS Config queries  |
| [Config DB](https://canarychecker.io/reference/configdb)                           | GA         | Custom config queries for Mission Control Config D           |
| [Kubernetes Resources](https://canarychecker.io/reference/kubernetes)              | GA         | Kubernetes resources that are missing or are in a non-ready state |
| **Backups**                                                  |            |                                                              |
| [GCP Databases](..refere)                                    | GA         | Backup freshness                                             |
| [Restic](https://canarychecker.io/reference/restic)                                | Beta       | Backup freshness and integrity                               |
| **Infrastructure**                                           |            |                                                              |
| [EC2](https://canarychecker.io/reference/ec2)                                      | GA         | Ability to launch new EC2 instances                          |
| [Kubernetes Ingress](https://canarychecker.io/reference/pod)                       | GA         | Ability to schedule and then route traffic via an ingress to a pod |
| [Docker/Containerd](https://canarychecker.io/reference/containerd)                 | Deprecated | Ability to push and pull containers via docker/containerd    |
| [Helm](https://canarychecker.io/reference/helm)                                    | Deprecated | Ability to push and pull helm charts                         |
| [S3 Protocol](https://canarychecker.io/reference/s3-protocol)                      | GA         | Ability to read/write/list objects on an S3 compatible object store |

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md)

Thank you to all our contributors !

<a href="https://github.com/flanksource/canary-checker/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=flanksource/canary-checker" />
</a>

## License

Canary Checker core (the code in this repository) is licensed under [Apache 2.0](https://raw.githubusercontent.com/flanksource/canary-checker/main/LICENSE) and accepts contributions via GitHub pull requests after signing a CLA.

The UI (Dashboard) is free to use with canary checker under a license exception of [Flanksource UI](https://github.com/flanksource/flanksource-ui/blob/main/LICENSE#L7)
