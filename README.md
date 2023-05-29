
<div align="center"> <img src="docs/canary-checker.png" height="64px"></img></div>
  <p align="center">Kubernetes operator for executing synthetic tests</p>
<p align="center">
<a href="https://github.com/flanksource/canary-checker/actions"><img src="https://github.com/flanksource/canary-checker/workflows/Test/badge.svg"></a>
<a href="https://goreportcard.com/report/github.com/flanksource/canary-checker"><img src="https://goreportcard.com/badge/github.com/flanksource/canary-checker"></a>
<img src="https://img.shields.io/github/license/flanksource/canary-checker.svg?style=flat-square"/>
<a href="https://canary-checker.docs.flanksource.com"> <img src="https://img.shields.io/badge/☰-Docs-lightgrey.svg"/> </a>
</p>

---

# Introduction

Canary Checker is a Kubernetes native multi-tenant synthetic monitoring system.  To learn more,  see the [docs](https://docs.flanksource.com/canary-checker/overview/).

# Features

* Built-in UI/Dashboard
* CRD based configuration and status reporting
* Prometheus Integration
* Runnable as a CLI for once-off checks or as a standalone server outside kubernetes
* Junit formatting for CI/CD integration
* Many built-in check types

## Getting started

The easiest way to get started with canary-checker is to run it as CLI, it will take specifications in a YAML / CRD format and execute them before returning. The CLI can be used within CI/CD platforms and also exports to JUnit XML reports.

1. Install the CLI

```bash
wget  https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker_linux_amd64   \
  -O /usr/bin/canary-checker && \
  chmod +x /usr/bin/canary-checker
```

2. Create a new  spec called `http.yaml`

```yaml
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: http-pass
spec:
  interval: 30
  http:
    - endpoint: https://httpstat.us/200
      thresholdMillis: 3000
      responseCodes: [201, 200, 301]
      responseContent: ""
      maxSSLExpiry: 7
```

3. Run the canary using:

```bash
canary-checker run http.yaml
```

[![asciicast](https://asciinema.org/a/N3jELGSn8HoRQHPpCdeK7MDBV.svg)](https://asciinema.org/a/N3jELGSn8HoRQHPpCdeK7MDBV)

### Junit Formating

Canary checker can export  JUnit formatted results for use in CI/CD pipelines

```bash
canary-checker run http.yaml -j -o results.xml
```

## Deploying as Kubernetes Operator

1. Deploy the operator

```bash
helm repo add flanksource https://flanksource.github.io/charts
helm repo update
helm install canary-checker-demo \
 --wait \
 -n canary-checker \
 --create-namespace flanksource/canary-checker \
 -f values.yaml
```

```yaml title="values.yaml"
flanksource-ui:
  ingress:
    host: canary-checker.127.0.0.1.nip.io
    annotations:
      kubernetes.io/ingress.class: nginx
      kubernetes.io/tls-acme: "true"
    tls:
      - secretName: canary-checker-tls
        hosts:
        - canary-checker.127.0.0.1.nip.io
```

2. Install a canary

```bash
kubectl apply -f https://raw.githubusercontent.com/flanksource/canary-checker/master/fixtures/minimal/http_pass_single.yaml
```

3. Check the results via the CLI

```bash
kubectl get canary
```

``` title="sample output"
NAME               INTERVAL   STATUS   LAST CHECK   UPTIME 1H        LATENCY 1H   LAST TRANSITIONED
http-pass-single   30         Passed   13s          18/18 (100.0%)   480ms        13s
```

### Dashboard

Canary checker comes with a built-in dashboard for displaying canary results, it can be turned off using `--set flanksource-ui.enabled=false`

![](https://github.com/flanksource/docs/blob/85bdd4875d0d3ded16b7aa6c132d423852fcad90/docs/images/dashboard-http-pass-canary.png?raw=true)

### Prometheus

The helm chart can install a `ServiceMonitor` for the prometheus operator, by enabling the serviceMonitor flag `--set serviceMonitor=true`

Metrics exposed by canary-checker:

| Metric                                         | Type      | Description                                 |
| ---------------------------------------------- | --------- | ------------------------------------------- |
| canary_check                                   | Guage     | Set to 0 when passing and 1 when failing    |
| canary_check_success_count                     | Counter   |                                             |
| canary_check_failed_count                      | Counter   |                                             |
| canary_check_info                              | Info      |                                             |
| canary_check_duration                          | Histogram | Histogram of canary durations               |
| **Pod Check Metrics**                          |           |                                             |
| canary_check_histogram{metric="creation_time"} | Histogram | Time for pod to be `Pending`                |
| canary_check_histogram{metric="delete_time"}   | Histogram | Time to delete pod                          |
| canary_check_histogram{metric="ingress_time"}  | Histogram | Time until ingress is returning requests    |
| canary_check_histogram{metric="request_time"}  | Histogram | Duration of http request once ingress is up |
| canary_check_histogram{metric="schedule_time"} | Histogram | Time for pod to be `Running`                |
| **HTTP Check Metrics**                         |           |                                             |
| canary_check_http_response_status              | Counter   | Response code counter for each endpoint     |
