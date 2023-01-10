

<div align="center"> <img src="docs/canary-checker.png" height="64px"></img></div>
  <p align="center">Kubernetes operator for executing synthetic tests</p>
<p align="center">
<a href="https://github.com/flanksource/canary-checker/actions"><img src="https://github.com/flanksource/canary-checker/workflows/Test/badge.svg"></a>
<a href="https://goreportcard.com/report/github.com/flanksource/canary-checker"><img src="https://goreportcard.com/badge/github.com/flanksource/canary-checker"></a>
<img src="https://img.shields.io/github/license/flanksource/canary-checker.svg?style=flat-square"/>
<a href="https://canary-checker.docs.flanksource.com"> <img src="https://img.shields.io/badge/☰-Docs-lightgrey.svg"/> </a>
</p>



---
<!--ts-->
- [Introduction](#introduction)
- [Features](#features)
- [Comparisons](#comparisons)
- [Quick Start](#quick-start)
- [Check Types](#check-types)
  - [DNS - Query a DNS server](#dns---query-a-dns-server)
  - [Containerd Pull - Pull an image using containerd](#containerd-pull---pull-an-image-using-containerd)
  - [Docker Pull - Pull an image using docker](#docker-pull---pull-an-image-using-docker)
  - [Docker Push - Create and push a docker image](#docker-push---create-and-push-a-docker-image)
  - [HTTP - Query an HTTP endpoint or namespace](#http---query-an-http-endpoint-or-namespace)
    - [displayTemplate](#displaytemplate)
  - [Helm - Build and push a helm chart](#helm---build-and-push-a-helm-chart)
  - [ICMP - Ping a destination and check for packet loss](#icmp---ping-a-destination-and-check-for-packet-loss)
  - [LDAP - Query a ldap(s) server](#ldap---query-a-ldaps-server)
  - [Namespace - Create a new kubernetes namespace and pod](#namespace---create-a-new-kubernetes-namespace-and-pod)
  - [Pod - Create a new pod and verify reachability](#pod---create-a-new-pod-and-verify-reachability)
  - [Postgres - Query a Postgresql DB using SQL](#postgres---query-a-postgresql-db-using-sql)
    - [displayTemplate](#displaytemplate-1)
  - [Mssql - Query a Mssql DB using SQL](#mssql---query-a-mssql-db-using-sql)
    - [displayTemplate](#displaytemplate-2)
  - [Elasticsearch - Query an Elasticsearch DB](#elasticsearch---query-an-elasticsearch-db)
  - [Redis - Execute ping against redis instance](#redis---execute-ping-against-redis-instance)
  - [S3 - Verify reachability and correctness of an S3 compatible store](#s3---verify-reachability-and-correctness-of-an-s3-compatible-store)
  - [Folder Check](#folder-check)
    - [S3 Bucket - Query the contents of an S3 bucket for freshness](#s3-bucket---query-the-contents-of-an-s3-bucket-for-freshness)
    - [GCS Bucket -  Query the contents of an GCS bucket for freshness](#gcs-bucket----query-the-contents-of-an-gcs-bucket-for-freshness)
    - [Smb - Verify Folder Freshness](#smb---verify-folder-freshness)
      - [server](#server)
  - [Restic - Query the contents of a Restic repository for backup freshness and integrity](#restic---query-the-contents-of-a-restic-repository-for-backup-freshness-and-integrity)
  - [Jmeter - Run the supplied JMX test plan against the specified host](#jmeter---run-the-supplied-jmx-test-plan-against-the-specified-host)
  - [SSL - Verify the expiry date of a SSL cert](#ssl---verify-the-expiry-date-of-a-ssl-cert)
  - [Exec - Execute commands or script on the host](#exec---execute-commands-or-script-on-the-host)
  - [TCP](#tcp)
  - [Junit](#junit)
    - [displayTemplate](#displaytemplate-3)
    - [displayTemplate](#displaytemplate-4)
  - [Display Types](#display-types)
    - [displayTemplate](#displaytemplate-5)
  - [Guide for Developers](#guide-for-developers)
<!--te-->

## Introduction

Canary Checker is a Kubernetes native multi-tenant synthetic monitoring system.  To learn more, please see the [official documentation](https://canary-checker.docs.flanksource.com).

## Features

* Built-in UI/Dashboard with multi-cluster aggregation
* CRD based configuration and status reporting
* Prometheus Integration
* Runnable as a CLI for once-off checks or as a standalone server outside kubernetes
* Many built-in check types

![dashboard](docs/images/ui01.png)
## Comparisons

| App                                                     | Comparison                                                   |
| ------------------------------------------------------- | ------------------------------------------------------------ |
| Prometheus                                              | canary-checker is not a replacement for prometheus, rather a companion. While prometheus provides persistent time series storage, canary-checker only has a small in-memory cache of recent checks.  Canary-checker also exposes metrics via `/metrics` that are scraped by prometheus. |
| Grafana                                                 | The built-in UI provides a mechanism to display check results across 1 or more instances without a dependency on grafana/prometheus running. The UI  will also display long-term graphs of check results by quering prometheus. |
| [Kuberhealthy](https://github.com/Comcast/kuberhealthy) | Very similar goals, but Kuberhealthy relies on external containers to implement checks and does not provide a UI or multi-cluster/instance aggregation. |
| [Cloudprober](https://cloudprober.org/)                 | Very similar goals, but Cloudprober is designed for very high scale, not multi-tenancy. Only has ICMP,DNS,HTTP,UDP built-in checks. |

## Quick Start

Before installing the Canary Checker, please ensure you have the [prerequisites installed](docs/prereqs.md) on your Kubernetes cluster.

The recommded method for installing Canary Checker is using [helm](https://helm.sh/)

### Install Helm

The following steps will install the latest version of helm

```bash
curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
chmod 700 get_helm.sh
./get_helm.sh
```

### Add the Flanksource helm repository

```bash
helm repo add flanksource https://flanksource.github.io/charts
helm repo update
```

### Configurable fields

See the [values file](chart/values.yaml) for the full list of configurable fields.  Mandatory configuration values are for the configuration of the database, and it is recommended to also configure the UI ingress.

#### DB

Canary Checker should optimally be run connected to a dedicated Postgres Server, but can run an embedded postgres instance for development and testing.

##### Embedded database (default):

| db.external.enabled | `false` (default) |
| db.embedded.storageClass | Set to name of a storageclass available in the cluster |
| db.embedded.storage | Set to volume of storage to request |

The Canary Checker statefulset will be configured to start an embedded postgres server in the pod, which stores data to a PVC

##### Fully automatic Postgres Server creation

| db.external.enabled | `true` |
| db.external.create  | `true` |
| db.external.storageClass | Set to name of a storageclass available in the cluster |
| db.external.storage | Set to volume of storage to request |

The helm chart will create a postgres server statefulset, with a random password and default port, along with a canarychecker database hosted on the server.

To specify a username and password for the chart-managed Postgres server, create a secret in the namespace that the chart will install to, named `postgres-connection`, which contains `POSTGRES_USER` and `POSTGRES_PASSWORD` keys.

#### Prexisting Server

In order to connect to an existing Postgres server, a database must be created on the server, along with a user that has <Not sure exactly what needs to be here> permissions

| db.external.enabled | `true` |
| db.external.create  | `false` |
| db.external.secretKeyRef.name | Set to name of name of secret that contains a key containging the postgres connection URI
| db.external.secretKeyRef.key | Set to the name of the key in the secret that contains the postgres connection URI |

The connection URI must be specified in the format `postgresql://"$user":"$password"@"$host"/"$database"`

#### Flanksource UI

By default, the canary checker only presents an API.  To view the data graphically, the Flankdource UI is required, and is installed by default. The UI should be configured to allow external access to via ingress

| flanksource-ui.ingress.host | URL at which the UI will be accessed |
| flanksource-ui.ingress.annotations | Map of annotations required by the ingress controller or certificate issuer |
| flanksource-ui.ingress.tls | Map of configuration options for TLS |

More details regarding ingress configuration can be found in the [kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/ingress/)

| flanksource-ui.backendURL | Required to be set to the name of the canary-checker service.  This will be the (RFC 1035 formatted) release name specified in the `helm install` command. |

Due to a limitation in Helm, there is no way to automatically propogate the generated service name to a child chart, and it must be aligned by the user.

### Deploy using Helm

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

## Check Types

### DNS - Query a DNS server

```yaml
dns:
  - server: 8.8.8.8
    port: 53
    query: "flanksource.com"
    querytype: "A"
    minrecords: 1
    exactreply: ["34.65.228.161"]
    timeout: 10
```

| Field | Description                             | Scheme | Required |
| ----- |-----------------------------------------| ------ | -------- |
| description | Description of check                    | string | Yes |
| server | Address of DNS server to query          | string | Yes |
| port | Port to query DNS server on             | int | Yes |
| query | Domain name to lookup                   | string |  |
| querytype | Record type to query                    | string | Yes |
| minrecords | Minimum records                         | int |  |
| exactreply | Expected exact match result(s)          | []string |  |
| timeout | Maximum timeout for DNS query           | int | Yes |
| thresholdMillis | Threshold response time from DNS server | int | Yes |

### Containerd Pull - Pull an image using containerd

This check will try to pull a Docker image from specified registry using containerd and then verify its checksum and size.

```yaml
containerdPull:
  - image: docker.io/library/busybox:1.31.1
    username:
    password:
    expectedDigest: 6915be4043561d64e0ab0f8f098dc2ac48e077fe23f488ac24b665166898115a
    expectedSize: 1219782
```

| Field          | Description                                                               | Scheme | Required |
| -------------- |---------------------------------------------------------------------------| ------ | -------- |
| description    | Description of check                                                      | string | Yes      |
| image          | Full path to image, including registry                                    | string | Yes      |
| auth | Username and password value, configMapKeyRef or SecretKeyRef for registry | Object | No |
| expectedDigest | Expected digest of the pulled image                                       | string | Yes      |
| expectedSize   | Expected size of the pulled image                                         | int64  | Yes      |

### Docker Pull - Pull an image using docker

This check will try to pull a Docker image from specified registry, verify it's checksum and size.

```yaml
docker:
  - image: docker.io/library/busybox:1.31.1
    auth:
      username:
        value: some-user
      password:
        value: some-password
    expectedDigest: 6915be4043561d64e0ab0f8f098dc2ac48e077fe23f488ac24b665166898115a
    expectedSize: 1219782
```

| Field | Description                                                               | Scheme | Required |
| ----- |---------------------------------------------------------------------------| ------ | -------- |
| description | Description of check                                                      | string | Yes |
| image | Full path to image, including registry                                    | string | Yes |
| auth | Username and password value, configMapKeyRef or SecretKeyRef for registry | Object | No |
| expectedDigest | Expected digest of the pulled image                                       | string | Yes |
| expectedSize | Expected size of the pulled image                                         | int64 | Yes |



### HTTP - Query an HTTP endpoint or namespace

```yaml
http:
  - endpoint: https://httpstat.us/200
    thresholdMillis: 3000
    responseCodes: [201,200,301]
    responseContent: ""
    maxSSLExpiry: 60
    displayTemplate: 'Response Code: [[.code]] Content: [[.content]]; Headers: [[.headers]]'
  - endpoint: https://httpstat.us/500
    thresholdMillis: 3000
    responseCodes: [500]
    responseContent: ""
    maxSSLExpiry: 60
  - endpoint: https://httpstat.us/500
    thresholdMillis: 3000
    responseCodes: [302]
    responseContent: ""
    maxSSLExpiry: 60
  - namespace: k8s-https-namespace
    thresholdMillis: 3000
    responseCodes: [200]
    responseContent: ""
    maxSSLExpiry: 60
    displayTemplate: 'Response Code: [[.code]]'
  - headers:
    - name: headerkey1
      value: headervalue1
    - name: headerkey2
      valueFrom:
        configMapRef:
          key: headervalue2
          name: header-configmap
    responseJSONContent:
      path: "$.headerkey1[0]"
      value: headervalue1
    endpoint: http://podinfo.127.0.0.1.nip.io/headers
    responseCodes:
      - 200
  - method: POST
    body: bodycontent
    responseContent: bodycontent
    endpoint: http://podinfo.127.0.0.1.nip.io/echo
    displayTemplate: 'Response Code: [[.code]] Content: [[.content]]; Headers: [[.headers]]'
    responseCodes:
      - 202
  - authentication:
      username:
        value: user
      password:
        valueFrom:
          secretKeyRef:
            name: authentication-secret
            key: password
    responsecodes:
      - 202
    endpoint: https://testloginserver.127.0.0.1.nip.io/login
```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description |  | string | Yes |
| endpoint | HTTP endpoint to monitor | string | Yes <sup>*</sup> |
| namespace | kubernetes namespace to monitor | string | Yes <sup>*</sup> |
| thresholdMillis | maximum duration in milliseconds for the HTTP request. It will fail the check if it takes longer. | int | Yes |
| responseCodes | expected response codes for the HTTP Request. | []int | Yes |
| responseContent | exact response content expected to be returned by the endpoint. | string | Yes |
| responseJSONContent | `path` and `value` to parse json responses. `path` is a [jsonpath](https://tools.ietf.org/id/draft-goessner-dispatch-jsonpath-00.html) string, `value` is the expected content at that path | JSONCheck | | 
| maxSSLExpiry | maximum number of days until the SSL Certificate expires. | int | Yes |
| method | specify GET (default) or POST method for HTTP call | string | | 
| body | body of HTTP method | string | |
| headers | array of key-value pairs to be passed as headers to the HTTP method.  Specified in the same manner as pod environment variables but without the support for pod spec references |   [[]kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | |
| authentication | `username` and `password` value, both of which are specified as [[]kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar), to be passed as authentication headers | *Authentication | |
| ntlm | if true, will change authentication protocol | bool | |
| displayTemplate | template to display server response in text (overrides default bar format for UI) | string | No |

<sup>*</sup> One of either endpoint or namespace must be specified, but not both.  Specify a namespace of `"*"` to crawl all namespaces.

#### displayTemplate

The fields for `displayTemplate` (see [Display Types]((#display-types))) are :

- `.code`: response code from the http server
- `.headers`: response headers
- `.content`: content from the http request



### ICMP - Ping a destination and check for packet loss

This test will check ICMP packet loss and duration.

```yaml
icmp:
  - endpoints:
      - https://google.com
      - https://yahoo.com
    thresholdMillis: 400
    packetLossThreshold: 50
    packetCount: 2
```

| Field | Description                                          | Scheme | Required |
| ----- |------------------------------------------------------| ------ | -------- |
| description | Description of check                                 | string | Yes |
| endpoint | Address to query using ICMP                          | string | Yes |
| thresholdMillis | Expected response time threshold in ms               | int64 | Yes |
| packetLossThreshold | Percent of total packets that are allowed to be lost | int64 | Yes |
| packetCount | Total number of packets to send per check            | int | Yes |


### LDAP - Query a ldap(s) server

The LDAP check will:

* bind using provided user/password to the ldap host. Supports ldap/ldaps protocols.
* search an object type in the provided bind DN.s

```yaml
ldap:
  - host: ldap://127.0.0.1:10389
    auth:
      username:
        value: uid=admin,ou=system
      password:
        value: secret
    bindDN: ou=users,dc=example,dc=com
    userSearch: "(&(objectClass=organizationalPerson))"
  - host: ldap://127.0.0.1:10389
    auth:
      username:
        value: uid=admin,ou=system
      password:
        value: secret
    bindDN: ou=groups,dc=example,dc=com
    userSearch: "(&(objectClass=groupOfNames))"
```

| Field | Description                                                                  | Scheme | Required |
| ----- |------------------------------------------------------------------------------| ------ | -------- |
| description | Description of check                                                         | string | Yes |
| host | URL of LDAP server to be qeuried                                             | string | Yes |
| auth | username and password value, configMapKeyRef or SecretKeyRef for LDAP server | Object | Yes |
| bindDN | BindDN to use in query                                                       | string | Yes |
| userSearch | UserSearch to use in query                                                   | string | Yes |
| skipTLSVerify | Skip check of LDAP server TLS certificates                                   | bool | Yes |


### Namespace - Create a new kubernetes namespace and pod

The namespace check will:

* Create a new namespace using the labels/annotations provided
* Create a new pod in the namespace using the provided PodSpec
* Expose the pod using the provided ingress URL
* Test an HTTP connection to the pod

```yaml
namespace:
  - namePrefix: "test-name-prefix-"
    labels:
      team: test
    annotations:
      "foo.baz.com/foo": "bar"
```

| Field | Description                                                                                                                          | Scheme | Required |
| ----- |--------------------------------------------------------------------------------------------------------------------------------------| ------ | -------- |
| description | Description of check                                                                                                                 | string | Yes |
| namespaceNamePrefix | Prefix string to identity namespace                                                                                                  | string | Yes |
| namespaceLabels | Metadata labels to apply to created namespace                                                                                        | map[string]string | Yes |
| namespaceAnnotations | Metadata annotations to apply to created namespace                                                                                   | map[string]string | Yes |
| podSpec | [Spec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#podspec-v1-core) of pod to be created in check namespace | string | Yes |
| scheduleTimeout | Maximum time between pod created and pod running                                                                                     | int64 | Yes |
| readyTimeout |                                                                                                                                      | int64 | Yes |
| httpTimeout | Maximum time to wait for an HTTP connection to the created pod                                                                       | int64 | Yes |
| deleteTimeout |                                                                                                                                      | int64 | Yes |
| ingressTimeout | Maximum time to create an ingress connected to the pod                                                                               | int64 | Yes |
| httpRetryInterval | Interval in ms to retry HTTP connections to the created pod                                                                          | int64 | Yes |
| deadline | Overall time before which an HTTP connection to the pod must be established                                                          | int64 | Yes |
| port | Port on which the created pod will serve traffic                                                                                     | int64 | Yes |
| path | Path on whcih the created pod will respond to requests                                                                               | string | Yes |
| ingressName | Name to use for the ingress object that will expose the created pod                                                                  | string | Yes |
| ingressHost | URL to be used by the ingress to expose the created pod                                                                              | string | Yes |
| expectedContent | Expected content of an HTTP response from the created pod                                                                            | string | Yes |
| expectedHttpStatuses | Expected HTTP status code of the response from the created pod                                                                       | []int64 | Yes |
| priorityClass | Pod priority class                                                                                                                   | string | Yes |


### Pod - Create a new pod and verify reachability

```yaml
pod:
  - name: golang
    namespace: default
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
    ingressName: hello-world-golang
    ingressHost: "hello-world-golang.127.0.0.1.nip.io"
    scheduleTimeout: 2000
    readyTimeout: 5000
    httpTimeout: 2000
    deleteTimeout: 12000
    ingressTimeout: 5000
    deadline: 29000
    httpRetryInterval: 200
    expectedContent: bar
    expectedHttpStatuses: [200, 201, 202]
```

| Field | Description                                                                                                       | Scheme | Required |
| ----- |-------------------------------------------------------------------------------------------------------------------| ------ | -------- |
| description | Description of the check                                                                                          | string | Yes |
| name | Name of the pod to be created                                                                                     | string | Yes |
| namespace | Namespace to create the pod in                                                                                    | string | Yes |
| spec | [Spec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#podspec-v1-core) of pod to be created | string | Yes |
| scheduleTimeout | Maximum time between pod created and pod running                                                                  | int64 | Yes |
| readyTimeout |                                                                                                                   | int64 | Yes |
| httpTimeout | Maximum time to wait for an HTTP connection to the created pod                                                    | int64 | Yes |
| deleteTimeout |                                                                                                                   | int64 | Yes |
| ingressTimeout | Maximum time to create an ingress connected to the pod                                                                                                                  | int64 | Yes |
| httpRetryInterval | Interval in ms to retry HTTP connections to the created pod                                                                                                                  | int64 | Yes |
| deadline | Overall time before which an HTTP connection to the pod must be established                                                          | int64 | Yes |
| port | Port on which the created pod will serve traffic                                                                                     | int64 | Yes |
| path | Path on whcih the created pod will respond to requests                                                                               | string | Yes |
| ingressName | Name to use for the ingress object that will expose the created pod                                                                  | string | Yes |
| ingressHost | URL to be used by the ingress to expose the created pod                                                                              | string | Yes |
| expectedContent | Expected content of an HTTP response from the created pod                                                                            | string | Yes |
| expectedHttpStatuses | Expected HTTP status code of the response from the created pod                                                                       | []int64 | Yes |
| priorityClass | Pod priority class                                                                                                                   | string | Yes |

### Postgres - Query a Postgresql DB using SQL

This check will try to connect to a specified Postgresql database, run a query against it and verify the results.

```yaml
postgres:
  - connection: "user=postgres password=mysecretpassword host=192.168.0.103 port=15432 dbname=postgres sslmode=disable"
    query: "SELECT * from names"
    resultsFunction: '[[ if index .results 0 "surname" | eq "khandelwal" ]]true[[else]]false[[end]]'
    displayTemplate: '[[ index .results 0 ]]'
```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | description for the test | string | No |
| connection | connection string to connect to the server | string | Yes |
| query | query that needs to be executed on the server  | string | Yes |
| resultsFunction | function that tests query output for pass/fail (must return boolean) | string | No |
| displayTemplate | template to display query results in text (overrides default bar format for UI) | string | No |

#### displayTemplate

The fields for `displayTemplate` are:

- `.results`: rows returned by the query

### Mssql - Query a Mssql DB using SQL

This check will try to connect to a specified Mssql database, run a query against it and verify the results.

```yaml
mssql:
  - connection: 'server=localhost;user id=sa;password=S0m3S3curep@sswd;port=1433;database=master'
    description: 'The mssql test'
    query: "SELECT * from names"
    resultsFunction: '[[ if index .results 0 "surname" | eq "khandelwal" ]]true[[else]]false[[end]]'
    displayTemplate: '[[ index .results 0 ]]'
```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| description | description for the test | string | No |
| connection | connection string to connect to the server | string | Yes |
| query | query that needs to be executed on the server  | string | Yes |
| resultsFunction | function that tests query output for pass/fail (must return boolean) | string | No |
| displayTemplate | template to display query results in text (overrides default bar format for UI) | string | No |

#### displayTemplate

The fields for `displayTemplate` (see [Display Types]((#display-types))) are:

- `.results`: rows returned by the query

### Redis - Execute ping against redis instance

This check will execute ping against the specified redis instance and check its availability

```yaml

redis:
  - addr: 'localhost:6379'
    description: 'The redis test'
    db: 0
```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| addr | host:port address. | string | Yes |
| db | database to be selected after connecting to the server. | int | Yes |
| description | description for canary | string | No |
| auth | username and password value, configMapKeyRef or SecretKeyRef for redis server | Object | No |

### Elasticsearch - Query an Elasticsearch DB

This check will try to connect to a specified Elasticsearch database, run a query against it and verify the results.

```yaml
elasticsearch:
  - url: http://elasticsearch-host.example.com:9200
    description: elasticsearch checker
    index: index_name
    query: |
      {
        "query": {
          "term": {
            "system.role": "api"
          }
        }
      }
    results: 3
```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| url | host:port address. | string | Yes |
| description | description for the test | string | No |
| index | index against which query should be ran | string | Yes |
| query | query that needs to be executed on the server  | string | Yes |
| results | number of expected hits | int | Yes |
| auth | username and password value, configMapKeyRef or SecretKeyRef for elasticsearch server | Object | No |


### S3 - Verify reachability and correctness of an S3 compatible store

This check will:

* list objects in the bucket to check for Read permissions
* PUT an object into the bucket for Write permissions
* download previous uploaded object to check for Get permissions

```yaml

s3:
  - buckets:
      - name: "test-bucket"
        region: "us-east-1"
        endpoint: "https://test-bucket.s3.us-east-1.amazonaws.com"
    secretKey: "<access-key>"
    accessKey: "<secret-key>"
    objectPath: "path/to/object"
```

| Field | Description                                      | Scheme | Required |
| ----- |--------------------------------------------------| ------ | -------- |
| description | Description for the check                        | string | Yes |
| bucket | Array of [Bucket](#bucket) objects to be checked | [Bucket](#bucket) | Yes |
| accessKey | AWS access Key to access Bucket                  | string | Yes |
| secretKey | AWS secret Key to access Bucket                  | string | Yes |
| objectPath | Path of object in bucket to                      | string | Yes |
| skipTLSVerify | skip TLS verify when connecting to s3            | bool | Yes |


### Folder Check

Folder Check provides an abstraction over checker related to folder.
Currently, used to perform the following checks:

| Field | Description                                                                   | Scheme | Required |
| ----- |-------------------------------------------------------------------------------| ------ | -------- |
| description | Description of check                                                          | string | No |
| name | name for the check                                                            | string | No |
| Icon | custom icon to display on the UI                                              | string | No |
| Test | Template to test the result against                                           | Object | No |
| Display | Template to display the result in                                             | Object | No |
| Path | Path to the object, needs to be prefixed with the protocol. See example below | string | Yes |
| SMBConnection |                                                                               | Object | No |
| AWSConnection |                                                                               | Object | No |
| GCPConnection |                                                                               | Object | No |
| Filter | Used to filter the objects                                                    | Object | No |
| FolderTest | Parameters to test the folder against                                         | Object | No |

#### S3 Bucket - Query the contents of an S3 bucket for freshness

This check will:

- search objects matching the provided object path pattern
- check that latest object is no older than provided `maxAge` value in seconds
- check that latest object size is not smaller than provided `minSize` value in bytes

For working with folder check, the bucket name needs to be prefixed with `s3://`
```yaml
folder:
    # Check for any backup not older than 7 days and min size 25 bytes
    - path: s3://refactoringtest-tarun
      awsConnection:
        accessKey:
          valueFrom:
            secretKeyRef:
              key: access-key
              name: aws
        secretKey:
          valueFrom:
            secretKeyRef:
              key: secret-key
              name: aws
        region: "ap-south-1"
      maxAge: 7d
      maxSize: 25b
```
#### GCS Bucket -  Query the contents of an GCS bucket for freshness
This check will:

- search objects matching the provided object path pattern
- check that latest object is no older than provided `maxAge` value in seconds
- check that latest object size is not smaller than provided `minSize` value in bytes

For working with folder check, the bucket name needs to be prefixed with `gcs://`
```yaml
folder:
   - description: gcs auth test
     path: gcs://somegcsbucket
     gcpConnection:
       credentials:
        valueFrom:
         configMapKeyRef:
           key: canary-checker-df017acc453c.json
           name: sa
     minAge: 1m
     maxAge: 5h
     minSize: 2M
     maxCount: 2
     minCount: 5
```

#### Smb - Verify Folder Freshness

This check connects to a samba server to check folder freshness. This check will:

- verify most recently modified file fulfills the `minAge` and `maxAge` constraints (each an optional bound)
- verify files present in the mount is more than `minCount`

```yaml
folder:
  - server: smb://192.168.1.9
    smbConnection:
      auth:
        username: 
          value: samba
        password:
          valueFrom:
            secretKeyRef:
              key: smb-password
              name: smb
      sharename: "Some Public Folder"
      searchPath: a/b/c
    minAge: 10h
    maxAge: 20h
    description: "Success SMB server"
```

Or with `server` in path format:

```yaml
folder:
   - server: '\\192.168.1.5\Some Public Folder\somedir'
     smbConnection:
       auth:
         username: 
          value: samba
         password: 
          value: password
       sharename: "sharename" #will be overwritten by 'Some Public Folder'
       searchPath: a/b/c #will be overwritten by 'somedir'
     minAge: 10h
     maxAge: 100h
     description: "Success SMB server"
```

##### server

The user can define server in two formats:

- host: `192.168.1.9`, `www.server.com`
- path: `\\www.server.com\e$\a\b\c`

For path format:

- `www.server.com` is the host
- `e$` is the sharename (overrides `sharename` field)
- `a/b/c` the sub-dir relative to mount representing the test root (overrides `searchPath` field)


### Restic - Query the contents of a Restic repository for backup freshness and integrity

This check will:

- query a Restic Repository for contents
- check the integrity and consistency of repo and data-blobs
- check bakup freshness

```yaml
restic:
    - repository: s3:http://minio.infra/restic-repo
      password: 
        value: S0M3p@sswd
      maxAge: 5h30m
      checkIntegrity: true
      accessKey: 
        value: some-access-key
      secretKey: 
        value: some-secret-key
      description: The restic test
```
  
| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| reposiory | the location of your restic repository | string | Yes |
| password | password value or valueFrom configMapKeyRef or SecretKeyRef to access your restic repository | string | Yes |
| maxAge   | the max age for backup allowed..eg: 5h30m | string | Yes |
| accessKey | access key value or valueFrom configMapKeyRef or SecretKeyRef to access your s3/minio bucket | string | No |
| secretkey | secret key value or valueFrom configMapKeyRef or SecretKeyRef to access your s3/minio bucket | string | No |
| description | the description about the canary | string | Yes |
| checkIntegrity | whether to check integrity for the specified repo | bool | No |
| caCert | path to ca-root crt in case of self-signed certificates is used | string | No |

### Jmeter - Run the supplied JMX test plan against the specified host

This check will execute the jmeter cli to execute the JMX test plan on the specified host.

> **Note:** JMeter is a memory hungry Java application and you will likely need to increase the default memory limit from 512Mi to 1-2Gi or higher depending on the complexity, count, and frequency of jmeter tests

```yaml
jmeter:
    - jmx:
        name: jmx-test-plan
        valueFrom:
          configMapKeyRef:
             key: jmeter-test.xml
             name: jmeter
      host: "some-host"
      port: 8080
      properties:
        - remote_hosts=127.0.0.1
      systemProperties:
        - user.dir=/home/mstover/jmeter_stuff
      description: "The Jmeter test"   
```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| jmx | configmap or Secret reference to get the JMX test plan | Object | Yes |
| host | the server against which test plan needs to be executed | String | No |
| port | the port on which the server is running | Int | No |
| properties | defines the local Jmeter properties | []String | No |
| systemProperties | defines the java system property | []String | No |
| description | the description of the canary | String | Yes |
| responseDuration | the duration under which all the test should pass | String | No |

### SSL - Verify the expiry date of a SSL cert

| Field | Description                                               | Scheme | Required |
| ----- |-----------------------------------------------------------| ------ | -------- |
| description | Description of check                                      | string | Yes |
| endpoint | HTTP endpoint to crawl                                    | string | Yes |
| maxSSLExpiry | maximum number of days until the SSL Certificate expires. | int | Yes |


### Exec - Execute commands or script on the host


| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| script | Inline commands or Absolute path to the script that needs to be executed | Yes |
| description | description for the check | string | No |
| display | format on how the result is going to be displayed on the console | Template | No |
| test | test against which result needs to be verified | Template | No |

### TCP

| Field | Description                                                 | Scheme | Required |
| ----- |-------------------------------------------------------------| ------ | -------- |
| description | Description of cGit heck                                    | string | Yes |
| endpoint | Endpoint to establish a TCP connection with                 | string | Yes |
| thresholdMillis | Maximum time in ms to wait for connection to be established | int64 | Yes |

### Junit

The check occurs on the specified container's completion and parses any junit test reports in the container (at the `testResults` path):

```yaml
junit:
  - testResults: "/tmp/junit-results/"
    description: "junit demo test"
    displayTemplate: 'Passed: [[.passed]]; Failed: [[.failed]]; Skipped: [[.skipped]]; Error: [[.error]]'
    spec:
      containers:
        - name: jes
          image: docker.io/tarun18/junit-test-pass
          command: ["/start.sh"]
```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| testResults | directory where the results will be published | string | Yes |
| description| description about the test | string | No |
| spec | pod specification | corev1.PodSpec | Yes |
| displayTemplate | template to display testResults results in text (default: `Passed: [[.passed]], Failed: [[.failed]]`) | string | No |

> **Note:** For this `corev1.PodSpec` implementation, only containers field is required.

#### displayTemplate

The fields for `displayTemplate` (see [Display Types]((#display-types))) are:

- `.passed`: number of tests passed
- `.failed`: number of tests failed
- `.skipped`: number of tests skipped
- `.error`: number of tests errored


#### displayTemplate

The fields for `displayTemplate` (see [Display Types]((#display-types))) are:

- `.age`: age of most recently modified file
- `.count`: number of files present in the mount

### Display Types

Most checks display a bar/historgram on the UI. Some checks display text data. Some depend on the fields specified in the check.

#### displayTemplate

Where display is text, the `displayTemplate` field allows a user to configure the output format. The `displayTemplate` field accepts a template with delimiter `[[` `]]` and supports all the functions of [gomplate](https://docs.gomplate.ca/).

Checks that currently have support for `displayTemplate` are:
- [s3Bucket](#s3-bucket---query-the-contents-of-an-s3-bucket-for-freshness)
- sql
  - [postgres](#postgres---query-a-postgresql-db-using-sql)
  - [mssql](#mssql---query-a-mssql-db-using-sql)
- [http](#http---query-an-http-endpoint-or-namespace)    
- [junit](#junit)
- [smb](#smb---verify-folder-freshness)

### Guide for Developers

This guide provides a step-by-step process for creating your local setup with the canary-checker: [dev Guide](docs/dev-guide.md).
