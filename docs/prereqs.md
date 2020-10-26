# Optional Prerequisites

Canary Checker can integrate with Prometheous and ships with a few defined promethues service monitors. While not strictly required this guide explains how to install the optional dependencies. 

## A Kubernetes Cluster

This is pretty obvious, but first [install](https://github.com/flanksource/karina) a kubernetes cluster and ensure you are able to run basic `kubectl` commands to create pods, services, and that cluster DNS is functioning properly. 

## Metrics Server

For many environments, Prometheus requires the [Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server) to get metrics from the worker nodes. How this is deployed is slightly different for different Kubernetes environments. [Here is an example of how to install the Metrics Server on AWS EKS](https://docs.aws.amazon.com/eks/latest/userguide/metrics-server.html).  If you are using GKE, you can skip this step as Prometheus is already installed using Stack Driver metrics. 

To ensure you are getting metrics run: 

```
kubectl get --raw /metrics
```


## Prometheus Operator
 
You can install the operator by cloning the [Prometheus Operator](https://github.com/prometheus-operator/kube-prometheus) repository with: 

```
git clone https://github.com/prometheus-operator/kube-prometheus
```

Find the appropriate release for your version of Kubernetes in the table.  For example, if you were using Kubernetes 1.17 (run `kubectl version` to see what you are running) you would see the [README](https://github.com/prometheus-operator/kube-prometheus/blob/master/README.md) shows I should be running `release-0.4`.  So to install we run: 

```
cd kube-prometheus
git branch -a
```
Here we see all the branch names.  To switch to the release branch run: 

```
git checkout remotes/origin/release-0.4
```

Now we can install the operator with: 

```
kubectl create -f manifests/setup
```

You should then be able to see custom resources, `servicemonitors` by running: 

```
kubectl get crd
```
And see there is a `servicemonitors.monitoring.coreos.com` custom resource definition. 

Once that is defined you can install the rest of the monitoring components: 

```
kubectl create -f manifests/
```

You'll be able to see all the resources defined in the `monitoring` namespace with: 

```
kubectl get pods -n monitoring
```

Output looks as follows: 

```
NAME                                   READY   STATUS    RESTARTS   AGE
alertmanager-main-0                    2/2     Running   0          3m15s
alertmanager-main-1                    2/2     Running   0          3m15s
alertmanager-main-2                    2/2     Running   0          3m15s
grafana-58dc7468d7-vvcnc               1/1     Running   0          3m12s
kube-state-metrics-765c7c7f95-kxddc    3/3     Running   0          3m12s
node-exporter-cnhm6                    2/2     Running   0          2m15s
node-exporter-vnh9r                    2/2     Running   0          3m13s
prometheus-adapter-5cd5798d96-j8xnn    1/1     Running   0          3m13s
prometheus-k8s-0                       3/3     Running   1          3m13s
prometheus-k8s-1                       3/3     Running   1          3m13s
prometheus-operator-5f75d76f9f-n9krn   1/1     Running   0          7m2s
```