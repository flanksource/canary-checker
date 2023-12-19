<div align="center">
  <picture>
    <source srcset="https://canarychecker.io/img/canary-checker-white.svg" media="(prefers-color-scheme: dark)">
    <img src="https://canarychecker.io/img/canary-checker.svg">
  </picture>
  
  <p>Kubernetes operator for executing synthetic tests</p>
  <p>
    <a href="https://github.com/flanksource/canary-checker/actions"><img src="https://github.com/flanksource/canary-checker/workflows/Test/badge.svg"></a>
    <a href="https://goreportcard.com/report/github.com/flanksource/canary-checker"><img src="https://goreportcard.com/badge/github.com/flanksource/canary-checker"></a>
    <img src="https://img.shields.io/github/license/flanksource/canary-checker.svg?style=flat-square"/>
    <a href="https://canarychecker.io"> <img src="https://img.shields.io/badge/☰-Docs-lightgrey.svg"/> </a>
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
* **Scriptable** - Go templates, Javascript and [Expr](https://github.com/antonmedv/expr) can be used to:
  * Evaluate whether a check is passing and severity to use when failing
  * Extract a user friendly error message
  * Transform and filter check responses into individual check results
* **Multi-Modal** - While designed as a Kubernetes Operator, canary checker can also run as a CLI and a server without K8s

## Getting Started

1. Install canary checker:

  ```shell
helm repo add flanksource https://flanksource.github.io/charts
helm repo update
helm install canary-checker
  ```

2. Create a new check:

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
canary-checker run canary.yaml
```

[![asciicast](https://asciinema.org/a/cYS6hlmX516JQeECHH7za3IDG.svg)](https://asciinema.org/a/cYS6hlmX516JQeECHH7za3IDG)

  ```shell
 kubectl apply -f canary.yaml
  ```

3. Check the status of the health check:

  ```shell
kubectl get canary
  ```

``` title="sample output"
NAME               INTERVAL   STATUS   LAST CHECK   UPTIME 1H        LATENCY 1H   LAST TRANSITIONED
http-check.        30         Passed   13s          18/18 (100.0%)   480ms        13s
```

## Getting Help

If you have any questions about canary checker:

* Read the [docs](https://canarychecker.io)
* Invite yourself to the [CNCF community slack](https://slack.cncf.io/) and join the [#canary-checker](https://cloud-native.slack.com/messages/canary-checker/) channel.
* Check out the [Youtube Playlist](https://www.youtube.com/playlist?list=PLz4F_KggvA58D6krlw433TNr8qMbu1aIU).
* File an [issue](https://github.com/flanksource/canary-checker/issues/new) - (We do provide user support via Github Issues, so don't worry  if your issue a real bug or not)

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
| [Azure Devops](https://canarychecker.io/reference)                                 |            |                                                              |
| **Integration Testing**                                      |            |                                                              |
| [JMeter](https://canarychecker.io/reference/jmeter)                                | Beta       | Runs and checks the result of a JMeter test                  |
| [JUnit / BYO](https://canarychecker.io/reference/junit)                            | Beta       | Run a pod that saves Junit test results                      |
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

## License

Canary Checker core (the code in this repository) is licensed under [Apache 2.0](https://raw.githubusercontent.com/flanksource/canary-checker/main/LICENSE) and accepts contributions via GitHub pull requests after signing a CLA.

The UI (Dashboard) is free to use with canary checker under a license exception of [Flanksource UI](https://github.com/flanksource/flanksource-ui/blob/main/LICENSE#L7)
