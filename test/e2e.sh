#!/bin/bash

set -e

export KUBECONFIG=~/.kube/config
export KARINA="karina -c $(pwd)/test/karina.yaml"
export DOCKER_API_VERSION=1.39
export CLUSTER_NAME=kind-test
export PATH=$(pwd)/.bin:$PATH
export ROOT=$(pwd)
export TEST_FOLDER=${TEST_FOLDER:-$1}
export TEST_REGEX=${TEST_REGEX:-$3}
SKIP_SETUP=${SKIP_SETUP:-$2}

if [[ "$1" == "" ]]; then
  echo "Usage ./test/e2e.sh TEST_FOLDER [--skip-setup] [RunRegex] "
  exit 1
fi

echo "Testing $TEST_FOLDER with $SKIP_SETUP"


if [[ "$SKIP_SETUP" != "--skip-setup" ]] ; then
  echo "::group::Provisioning"
  if [[ ! -e .certs/root-ca.key ]]; then
    $KARINA ca generate --name root-ca --cert-path .certs/root-ca.crt --private-key-path .certs/root-ca.key --password foobar  --expiry 1
    $KARINA ca generate --name ingress-ca --cert-path .certs/ingress-ca.crt --private-key-path .certs/ingress-ca.key --password foobar  --expiry 1
    $KARINA ca generate --name sealed-secrets --cert-path .certs/sealed-secrets-crt.pem --private-key-path .certs/sealed-secrets-key.pem --password foobar  --expiry 1
  fi
  if $KARINA provision kind-cluster -v ; then
    echo "::endgroup::"
  else
    echo "::endgroup::"
    exit 1
  fi

  kubectl config use-context kind-$CLUSTER_NAME

  echo "::group::Deploying Base"
  $KARINA deploy bootstrap -vv
  echo "::endgroup::"
fi

if [ -e $TEST_FOLDER/_setup.sh ]; then
  sh $TEST_FOLDER/_setup.sh || echo Setup failed, attempting tests anyway
fi
if [ -e $TEST_FOLDER/_setup.yaml ]; then
  kubectl apply -f $TEST_FOLDER/_setup.yaml
fi
if [ -e $TEST_FOLDER/main.go ]; then
  go run $TEST_FOLDER/main.go
fi

cd $ROOT/test

if [[ "$TEST_REGEX" != "" ]]; then
  EXTRA=" -test.run TestRunChecks/${TEST_REGEX}.* "
fi

echo "::group::Compiling tests"
# first compile the test binary
go test ./... -v -c
echo "::endgroup::"
echo "::group::Testing"
USER=$(whoami)
sudo DOCKER_API_VERSION=1.39 --preserve-env=KUBECONFIG,TEST_FOLDER ./test.test -test.v --test-folder $TEST_FOLDER $EXTRA  2>&1
echo "::endgroup::"
