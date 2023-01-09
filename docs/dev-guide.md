# Dev Guide

The guide provides steps to configure your local setup with running canary-checker.

## Set up a Kubernetes Cluster

Before proceeding one must have access to a functioning Kubernetes cluster.

In this guide we'll be working with [kind](https://kind.sigs.k8s.io/).

### Create a cluster

- Install latest version of [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation).
- Create the cluster with `kind create cluster --name <cluster-name>`.

### Install Metrics Server and Prometheus

For installing and configuring metrics server and prometheus please see the [prereqs](prereqs.md).

## Install Node

The built-in dashboard of canary-checker uses node v12. Developers will need to install the compatible version:

- Install [nvm](https://github.com/nvm-sh/nvm) to install and manage different versions of node.
- After nvm is installed one can install node v12 by running `nvm install v12`.

## Build the canary-checker binary

**Note**: The following commands needs to be run from the root of canary-checker repo.

For most development work, it makes sense to build the binary and run the operator directly, rather than building and running a container image. With this in mind, this is how we can build and use the binary:

- Based on your current distro build the binaries: `make linux|darwin|windows`
- After building the binary one needs to apply the canary-checker CRD inside the cluster: `kubectl apply -f config/deploy/crd.yaml`
- Once the CRDs are present in the cluster one can start the operator using the binary by: `./.bin/canary-checker-amd64 operator`.
  - Please note that binary file naming convention is different on different operating systems. Look under `./.bin` to find yours.

Now you can deploy the canary-checks in any namespace of the cluster and they'll be reconciled by the operator.

You can test if your operator is working correctly by deploying a sample Canary (by `kubectl` applying the following template):

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: http-pass
spec:
  interval: 30
  http:
    - name: http
      endpoint: https://httpstat.us/200
      thresholdMillis: 3000
      responseCodes: [201, 200, 301]
      responseContent: ""
      maxSSLExpiry: 7
EOF
```

You can test the canary status by running: `kubectl get canaries.canaries.flanksource.com`

Sample output:

```
kubectl get canaries.canaries.flanksource.com
NAME        INTERVAL   STATUS   MESSAGE   UPTIME 1H    LATENCY 1H   LAST TRANSITIONED   LAST CHECK
http-pass   30         Passed             1/1 (100%)   500ms                            7s
```
