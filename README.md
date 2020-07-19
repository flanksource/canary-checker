

<h1 align="center">Canary Checker</h1>
  <p align="center">Tool for executing checks and reporting results via prometheus</p>
<p align="center">
<a href="https://circleci.com/gh/flanksource/canary-checker"><img src="https://circleci.com/gh/flanksource/canary-checker.svg?style=svg"></a>
<a href="https://goreportcard.com/report/github.com/flanksource/canary-checker"><img src="https://goreportcard.com/badge/github.com/flanksource/canary-checker"></a>
<img src="https://img.shields.io/github/license/flanksource/canary-checker.svg?style=flat-square"/>
<a href="https://canary-checker.docs.flanksource.com"> <img src="https://img.shields.io/badge/☰-Docs-lightgrey.svg"/> </a>
</p>

---
## Features

* Built-in UI/Dashboard with multi-cluster aggregation
* CRD based configuration and status reporting
* Prometheus Integration
* Runnable as a CLI for once-off checks or as a standalone server outside kubernetes
* Built-in check types
  * **http** - query a HTTP url and verify response code and content
  * **dns** - query a DNS server and verify results
  * **docker** - pull a docker image and verify size and digest
  * **dockerPush** - push a docker image
  * **helm** - push and pull a helm chart
  * **s3** - List, Put, and Get an object in an S3 bucket
  * **s3Bucket** - query the contents on a bucket for freshness and size, useful for verifying backups have been created
  * **tcp** - connect to a TCP port
  * **pod** - schedule a pod in kubernetes cluster
  * **namespace** - create and namespace and then run the pod check
  * **ldap** - query a ldap server for an object
  * **ssl** - check ssl certificate expiry
  * **icmp** - ping an IP and verify latency and packet loss threshold
  * **postgres** - query a postgres database for a result

## Getting Started

```bash
# install the operator using kustomize
kustomize build github.com/flanksource/canary-checker//config | kubectl apply -f -
# deploy a sample canary
kubectl apply -f https://raw.githubusercontent.com/flanksource/canary-checker/master/fixtures-crd/http_pass.yaml
# check the results of the canary
kubectl get canary
```
`sample output`
```
NAMESPACE         NAME   INTERVAL   STATUS   MESSAGE   UPTIME 1H      LATENCY 1H   LAST TRANSITIONED   LAST CHECK
platform-system   dns    30         Passed             0/2 (0%)                                        6s
platform-system   lan    30         Passed             12/12 (100%)   1033         139m                6s
platform-system   ldap   30         Passed             5/5 (100%)     323                              1s
platform-system   pod    120        Passed             1/2 (50%)      10904        45m                 24s
platform-system   s3     30         Passed             5/5 (100%)     1091         5m35s               5s
```

`http_pass.yaml`

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
