

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

Canary Checker is a Kubernetes native multi-tenant synthetic monitoring system.  To learn more, please see the [official documentation](https://canary-checker.docs.flanksource.com).

# Features

* Built-in UI/Dashboard with multi-cluster aggregation
* CRD based configuration and status reporting
* Prometheus Integration
* Runnable as a CLI for once-off checks or as a standalone server outside kubernetes
* Many built-in check types


# Quick Start

Before installing the Canary Checker, please ensure you have the [prerequisites installed](docs/prereqs.md) on your Kubernetes cluster.

The recommended method for installing Canary Checker is using [helm](https://helm.sh/)

## Install Helm

The following steps will install the latest version of helm

```bash
curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
chmod 700 get_helm.sh
./get_helm.sh
```

## Add the Flanksource helm repository

```bash
helm repo add flanksource https://flanksource.github.io/charts
helm repo update
```

## Configurable fields

See the [values file](chart/values.yaml) for the full list of configurable fields.  Mandatory configuration values are for the configuration of the database, and it is recommended to also configure the UI ingress.

## DB

Canary Checker should optimally be run connected to a dedicated Postgres Server, but can run an embedded postgres instance for development and testing.

### Embedded database (default):

|                     |                   |
|---------------------|-------------------|
| db.external.enabled | `false` (default) |
| db.embedded.storageClass | Set to name of a storageclass available in the cluster |
| db.embedded.storage | Set to volume of storage to request |

The Canary Checker statefulset will be configured to start an embedded postgres server in the pod, which stores data to a PVC

To connect to the embedded database: 

```shell
kubectl port-forward canary-checker-0 6432:6432 
psql -U postgres localhost -p 6432 canary with password postgres #password will be postgres
```



### Fully automatic Postgres Server creation

|                     |                   |
|---------------------|-------------------|
| db.external.enabled | `true` |
| db.external.create  | `true` |
| db.external.storageClass | Set to name of a storageclass available in the cluster |
| db.external.storage | Set to volume of storage to request |

The helm chart will create a postgres server statefulset, with a random password and default port, along with a canarychecker database hosted on the server.

To specify a username and password for the chart-managed Postgres server, create a secret in the namespace that the chart will install to, named `postgres-connection`, which contains `POSTGRES_USER` and `POSTGRES_PASSWORD` keys.

### External Postgres Server

In order to connect to an existing Postgres server, a database must be created on the server, along with a user that has administrator permissions for the database.git 

|                     |                   |
|---------------------|-------------------|
| db.external.enabled | `true` |
| db.external.create  | `false` |
| db.external.secretKeyRef.name | Set to name of name of secret that contains a key containging the postgres connection URI |
| db.external.secretKeyRef.key | Set to the name of the key in the secret that contains the postgres connection URI |

The connection URI must be specified in the format `postgresql://"$user":"$password"@"$host"/"$database"`

## Flanksource UI

The canary checker itself only presents an API.  To view the data graphically, the Flanksource UI is required, and is installed by default. The UI should be configured to allow external access to via ingress

|                     |                   |
|---------------------|-------------------|
| flanksource-ui.ingress.host | URL at which the UI will be accessed |
| flanksource-ui.ingress.annotations | Map of annotations required by the ingress controller or certificate issuer |
| flanksource-ui.ingress.tls | Map of configuration options for TLS |

More details regarding ingress configuration can be found in the [kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/ingress/)

|                     |                   |
|---------------------|-------------------|
| flanksource-ui.backendURL | Required to be set to the name of the canary-checker service.  The name will default to 'canary-checker' unless `nameOverride` is specified.  If `nameOverride is set, `backendURL` must be set to the same value |

Due to a limitation in Helm, there is no way to automatically propogate the generated service name to a child chart, and it must be aligned by the user.

## Deploy using Helm

To install into a new `canary-checker` namespace, run

```bash
helm install canary-checker-demo --wait -n canary-checker --create-namespace flanksource/canary-checker -f values.yaml
```

where `values.yaml` contains the configuration options detailed above.  eg

```yaml
db:
  external: true
  create: true
  storageClass: default
  storage: 30Gi
flanksource-ui:
  ingress:
    host: canary-checker.flanksource.com
    annotations:
      kubernetes.io/ingress.class: nginx
      kubernetes.io/tls-acme: "true"
    tls:
      - secretName: canary-checker-tls
        hosts:
        - canary-checker.flanksource.com
```

### Deploy a sample Canary

```bash
kubectl apply -f https://raw.githubusercontent.com/flanksource/canary-checker/master/fixtures-crd/http_pass.yaml
```

### Check the results of the Canary

```bash
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
