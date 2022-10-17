#!/bin/bash

# The script tests the push subcommand as well as postgres convectivity for canary-checker.

set -e

export KUBECONFIG=~/.kube/config
export KARINA="karina -c $(pwd)/test/karina.yaml"
export DOCKER_API_VERSION=1.39
export CLUSTER_NAME=kind-test
export PATH=$(pwd)/.bin:$PATH
export ROOT=$(pwd)

echo "::group::Provisioning"
if [[ ! -e .certs/root-ca.key ]]; then
$KARINA ca generate --name root-ca --cert-path .certs/root-ca.crt --private-key-path .certs/root-ca.key --password foobar  --expiry 1
$KARINA ca generate --name ingress-ca --cert-path .certs/ingress-ca.crt --private-key-path .certs/ingress-ca.key --password foobar  --expiry 1
$KARINA ca generate --name sealed-secrets --cert-path .certs/sealed-secrets-crt.pem --private-key-path .certs/sealed-secrets-key.pem --password foobar  --expiry 1
fi

if $KARINA provision kind-cluster -e name=$CLUSTER_NAME -v ; then
echo "::endgroup::"
else
echo "::endgroup::"
exit 1
fi

kubectl config use-context kind-$CLUSTER_NAME

echo "::group::Operator Setup"

export IMG=docker.io/flanksource/canary-checker:test 
make docker
kind load docker-image $IMG --name kind-$CLUSTER_NAME


helm dependency build $ROOT/chart
helm install -f $ROOT/test/values.yaml canary-checker $ROOT/chart -n default


sleep 90


kubectl get po -n default
kubectl logs -n default -l app.kubernetes.io/name=canary-checker 

echo "::endgroup::"