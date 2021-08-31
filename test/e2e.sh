#!/bin/bash

set -e

export KARINA="karina -c test/config.yaml"
export KUBECONFIG=~/.kube/config
export DOCKER_API_VERSION=1.39
export CLUSTER_NAME=kind-test
export PATH=$(pwd)/.bin:$PATH

KARINA_SETUP=${KARINA_SETUP:-true}
WAIT=${WAIT:-true}
RESTIC=${RESTIC:-true}
UI=${UI:-true}


if $KARINA_SETUP ; then
  echo "::group::Provisioning"
  if [[ ! -e .certs/root-ca.key ]]; then
    $KARINA ca generate --name root-ca --cert-path .certs/root-ca.crt --private-key-path .certs/root-ca.key --password foobar  --expiry 1
    $KARINA ca generate --name ingress-ca --cert-path .certs/ingress-ca.crt --private-key-path .certs/ingress-ca.key --password foobar  --expiry 1
    $KARINA ca generate --name sealed-secrets --cert-path .certs/sealed-secrets-crt.pem --private-key-path .certs/sealed-secrets-key.pem --password foobar  --expiry 1
  fi
  if $KARINA provision kind-cluster --trace -vv ; then
    echo "::endgroup::"
  else
    echo "::endgroup::"
    exit 1
  fi

  kubectl config use-context kind-$CLUSTER_NAME

  echo "::group::Deploying Base"
  $KARINA deploy bootstrap -vv
  echo "::endgroup::"
  echo "::group::Deploying Stubs"
  $KARINA deploy apacheds
  echo "::endgroup::"
  echo "::deploy monitoring::"
  $KARINA deploy monitoring
  #$KARINA test stubs --wait=480 -v 5
  echo "::group::Setting up test environment"
  kubectl -n ldap delete svc apacheds
  $KARINA apply setup.yml
  echo "::endgroup::"
fi

echo "::group::Waiting for environment"

export DOCKER_USERNAME=test
export DOCKER_PASSWORD=password

if $RESTIC ; then
  #Verify
  restic version
  # Initialize Restic Repo
  # Do not fail if it already exists
  RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker init || true
  #take some backup in restic
  RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker backup $(pwd)
fi;

if $WAIT ; then
  wait4x tcp 127.0.0.1:30636 -t 120s -i 5s || true
  wait4x tcp 127.0.0.1:30389 || true
  wait4x tcp 127.0.0.1:32432 || true
  wait4x tcp 127.0.0.1:32004 || true
  wait4x tcp 127.0.0.1:32010 || true
  wait4x tcp 127.0.0.1:32018 || true
  wait4x tcp 127.0.0.1:32015 || true
fi

echo "::endgroup::"

if $UI ; then
  echo "::group::Building UI"
  make ui
  echo "::endgroup::"
fi


kubectl apply -R -f test/nested-canaries/
kubectl create secret generic aws-credentials --from-literal=AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID --from-literal=AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY -n podinfo-test -o yaml --dry-run | kubectl apply -n podinfo-test -f -


cd test

if [[ "$1" != "" ]]; then
  EXTRA=" -test.run TestRunChecks/${1}.* "
fi

echo "::group::Compiling tests"
# first compile the test binary
go test ./... -v -c
echo "::endgroup::"
echo "::group::Testing"
USER=$(whoami)
sudo DOCKER_API_VERSION=1.39 --preserve-env=KUBECONFIG,TEST_FOLDER ./test.test  -test.v $EXTRA $2 2>&1
echo "::endgroup::"
