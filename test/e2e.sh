#!/bin/bash

set -e

export KUBECONFIG=~/.kube/config
export KARINA="karina -c $(pwd)/test/karina.yaml"
export DOCKER_API_VERSION=1.39
export CLUSTER_NAME=kind-test
export PATH=$(pwd)/.bin:$PATH
export ROOT=$(pwd)
export TEST_FOLDER=${TEST_FOLDER:-$1}
export DOMAIN=${DOMAIN:-127.0.0.1.nip.io}
export TELEPRESENCE_USE_DEPLOYMENT=0

SKIP_SETUP=${SKIP_SETUP:-false}
SKIP_KARINA=${SKIP_KARINA:-false}
SKIP_TELEPRESENCE=${SKIP_TELEPRESENCE:-false}

if [[ "$1" == "" ]]; then
  echo "Usage ./test/e2e.sh TEST_FOLDER  [RunRegex] [--skip-setup] [--skip-karina] [--skip-telepresence] [--skip-all] "
  exit 1
fi

if [[ "$*" == *"--skip-setup"* ]]; then
  SKIP_SETUP=true
fi
if [[ "$*" == *"--skip-karina"* ]]; then
  SKIP_KARINA=true
fi
if [[ "$*" == *"--skip-telepresence"* ]]; then
  SKIP_TELEPRESENCE=true
fi
if [[ "$*" == *"--skip-all"* ]]; then
  SKIP_TELEPRESENCE=true
  SKIP_KARINA=true
  SKIP_SETUP=true
fi

echo "Testing $TEST_FOLDER with setup=$SKIP_SETUP karina=$SKIP_KARINA"

if [[ "$SKIP_KARINA" != "true" ]] ; then
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

  echo "::group::Deploying Base"
  $KARINA deploy bootstrap -vv
  echo "::endgroup::"
fi

_DOMAIN=$(kubectl get cm -n quack quack-config -o json | jq -r ".data.domain" || echo)
if [[ "$_DOMAIN" != "" ]]; then
  echo Using domain: $_DOMAIN
  export DOMAIN=$_DOMAIN
fi

if [ "$SKIP_SETUP" != "true" ]; then
  echo "::group::Setting up"

  if [ -e $TEST_FOLDER/_karina.yaml ]; then
    $KARINA deploy phases --stubs --monitoring --apacheds --minio -c $(pwd)/$TEST_FOLDER/_karina.yaml -vv
  fi

  if [ -e $TEST_FOLDER/_setup.sh ]; then
    bash $TEST_FOLDER/_setup.sh || echo Setup failed, attempting tests anyway
  fi

  if [ -e $TEST_FOLDER/_setup.yaml ]; then
    $KARINA apply $(pwd)/$TEST_FOLDER/_setup.yaml -v
  fi

  if [ -e $TEST_FOLDER/../_setup.yaml ]; then
    $KARINA apply $(pwd)/$TEST_FOLDER/../_setup.yaml -v
  fi

  if [ -e $TEST_FOLDER/_post_setup.sh ]; then
    bash $TEST_FOLDER/_post_setup.sh || echo Post setup failed, attempting tests anyway
  fi

  if [ -e $TEST_FOLDER/main.go ]; then
    cd $TEST_FOLDER
    go run main.go
  fi
  echo "::endgroup::"
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

if [[ "$SKIP_TELEPRESENCE" != "true" ]]; then
  telepresence="telepresence --mount false -m vpn-tcp --namespace default --run"
fi
cmd="$telepresence ./test.test -test.v --test-folder $TEST_FOLDER $EXTRA"
echo $cmd
DOCKER_API_VERSION=1.39
set +e -o pipefail
sudo --preserve-env=KUBECONFIG,TEST_FOLDER,DOCKER_API_VERSION $cmd  2>&1 | tee test.out
code=$?
echo "return=$code"
sudo chown $USER test.out
cat test.out | go-junit-report > test-results.xml
echo "::endgroup::"
exit $code
