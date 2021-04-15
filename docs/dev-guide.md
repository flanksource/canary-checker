# Dev Guide
The guide provides steps to configure your local setup with running canary-checker


## Setup a Kubernetes Cluster
Before proceeding one must have access to a functioning Kubernetes cluster

In this guide we'll be working with [kind](https://kind.sigs.k8s.io/)

### Create a cluster

- Install latest version of [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- Create the cluster with `kind create cluster --name <cluster-name>`

### Install Metrics Server and Prometheus

For installing and configuring metrics server and prometheus please see the [prereqs](prereqs.md)



## Install Node
The built-in dashboard of canary-checker uses node v12. So developer need to install the compatible version

- Install [nvm](https://github.com/nvm-sh/nvm) to install and manage different version of node
- After nvm is installed one can install node v12 by running `nvm install v12`



## Build the canary-checker binary
In most of the development work it makes sense to just build the binary and run th operator directly. Compared to the alternative approach of building the container image and using it

- Based on your current distro build the binaries: `make linux|darwin-arm64|darwin-amd64|windows`
- After building the binary one needs to apply the canary-checker CRD inside the cluster. `kubectl apply -f config/deploy/crd.yaml`
- Once the CRDs are present in the cluster one can start the operator using the binary by: `./.bin/canary-checker-amd64 operator`
The above command will also start the canary-dashboard on `0.0.0.0:8080`

Now you can deploy the canary-checks in any namespace of the cluster and they'll be reconciled by the operator  

Note: The above command needs to be run from the root of canary-checker repo
