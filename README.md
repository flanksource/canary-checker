

<h1 align="center">Canary Checker</h1>
  <p align="center">Tool for executing checks and reporting results via prometheus</p>
<p align="center">
<a href="https://circleci.com/gh/flanksource/canary-checker"><img src="https://circleci.com/gh/flanksource/canary-checker.svg?style=svg"></a>
<a href="https://goreportcard.com/report/github.com/flanksource/canary-checker"><img src="https://goreportcard.com/badge/github.com/flanksource/canary-checker"></a>
<img src="https://img.shields.io/github/license/flanksource/canary-checker.svg?style=flat-square"/>
<a href="https://canary-checker.docs.flanksource.com"> <img src="https://img.shields.io/badge/☰-Docs-lightgrey.svg"/> </a>
</p>
---
* **http** - query a HTTP url and verify response code and content
* **dns** - query a DNS server and verify results
* **docker** - pull a docker image and verify size and digest
* **dockerPush** - push a docker image
* **helm** - push and pull a helm chart
* **s3** - List, Put, and Get an object in an S3 bucket
* **s3Bucket** - query the contents on a bucket for freshness and size, useful for verifying backups have been created
* **tcp** - connect to a TCP port
* **pod** - schedule a pod in kubernetes cluster
* **pod_and_ingress** - schedule a pod in kubernetes cluster and verify it is accessible via an ingress
* **ldap** - query a ldap server for an object
* **ssl** - check ssl certificate expiry
* **icmp** - ping an IP and verify latency and packet loss threshold
* **postgres** - query a postgres database for a result



### Getting Started


```
Usage:

	canary-checker serve [flags]

Flags:

      --failureThreshold int   Default Number of consecutive failures required to fail a check (default 2)
      --httpPort int           Port to expose a health dashboard  (default 8080)
      --interval uint          Default interval (in seconds) to run checks on (default 30)

Global Flags:

  -c, --configfile string   Specify configfile
  -v, --loglevel count      Increase logging level
```



The config file is YAML formatted:

```yaml
http:
  - endpoints:
      - https://httpstat.us/200
    thresholdMillis: 3000
    responseCodes: [201,200,301]
    responseContent: ""
    maxSSLExpiry: 7
```

