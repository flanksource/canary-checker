#!/bin/bash

set -e

export KUBECONFIG=~/.kube/config
export KARINA="karina -c $(pwd)/test/karina.yaml"
export CLUSTER_NAME=kind-test
export PATH=$(pwd)/.bin:/usr/local/bin:$PATH
export ROOT=$(pwd)
export TEST_FOLDER=${TEST_FOLDER:-$1}
export TEST_BINARY=./test.test
SKIP_SETUP=${SKIP_SETUP:-false}
SKIP_KARINA=${SKIP_KARINA:-false}

if [[ "$1" == "" ]]; then
  echo "Usage ./test/e2e.sh TEST_FOLDER  [RunRegex] [--skip-setup] [--skip-karina] [--skip-all] "
  exit 1
fi

if [[ "$*" == *"--skip-setup"* ]]; then
  SKIP_SETUP=true
fi
if [[ "$*" == *"--skip-karina"* ]]; then
  SKIP_KARINA=true
fi
if [[ "$*" == *"--skip-all"* ]]; then
  SKIP_KARINA=true
  SKIP_SETUP=true
fi

echo "Testing $TEST_FOLDER with skip_setup=$SKIP_SETUP skip_karina=$SKIP_KARINA"

if [[ "$SKIP_KARINA" != "true" ]] ; then
  echo "::group::Provisioning"
  if [[ ! -e .certs/root-ca.key ]]; then
    $KARINA ca generate --name root-ca --cert-path .certs/root-ca.crt --private-key-path .certs/root-ca.key --password foobar  --expiry 1
    $KARINA ca generate --name ingress-ca --cert-path .certs/ingress-ca.crt --private-key-path .certs/ingress-ca.key --password foobar  --expiry 1
    $KARINA ca generate --name sealed-secrets --cert-path .certs/sealed-secrets-crt.pem --private-key-path .certs/sealed-secrets-key.pem --password foobar  --expiry 1
  fi

  if ! kind get clusters | grep $CLUSTER_NAME; then
    if $KARINA provision kind-cluster -e name=$CLUSTER_NAME -v ; then
      echo "::endgroup::"
    else
      echo "::endgroup::"
      exit 1
    fi
  fi

  echo "::group::Deploying Base"
  $KARINA deploy bootstrap -vv --prune=false
  echo "::endgroup::"
fi


if [ "$SKIP_SETUP" != "true" ]; then
  echo "::group::Setting up"
  export GOOS=linux
  export GOARCH=amd64
  kubectl create ns canaries || true
  if [ -e $TEST_FOLDER/_karina.yaml ]; then
    $KARINA deploy phases --stubs --monitoring --apacheds --minio -c $(pwd)/$TEST_FOLDER/_karina.yaml -vv --prune=false
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

image=ubuntu

if [[ -e $TEST_FOLDER/_image ]]; then
  image=$(cat $TEST_FOLDER/_image )
  echo Using image: $image
fi

cd $ROOT/test

if [[ "$TEST_REGEX" != "" ]]; then
  EXTRA=" -test.run TestRunChecks/${TEST_REGEX}.* "
fi

if [[ ! -e $TEST_BINARY ]]; then
  echo "::group::Compiling tests"
  make build
  echo "::endgroup::"
fi
echo "::group::Testing"

rm test.out test-results.xml test-results.json || true
runner=ginkgo-$(date +%s)

if [ "$SKIP_SETUP" != "true" ]; then
  k="kubectl -n default"
  $k create clusterrolebinding ginkgo-default  --clusterrole=cluster-admin --serviceaccount=default:default || true
  $k run $runner --image $image   --command -- bash -c 'sleep 3600'
  function cleanup {
    $k delete po $runner --wait=false
    $k delete clusterrolebinding ginkgo-default
  }
  trap cleanup EXIT
  $k wait --for=condition=Ready pod/$runner  --timeout=5m
  echo "Copying $TEST_FOLDER to $runner"
  $k cp ../$TEST_FOLDER $runner:/tmp/fixtures
  echo "Copying $TEST_BINARY to $runner"
  $k cp  $TEST_BINARY $runner:/tmp/test
  set +e
  $k exec -it $runner -- bash -c  "/tmp/test --test-folder /tmp/fixtures $EXTRA  --ginkgo.junit-report /tmp/test-results.xml --ginkgo.vv  -verbose 1"
  out=$?
  $k cp  $runner:/tmp/test-results.xml test-results.xml
else
  $TEST_BINARY --test-folder $TEST_FOLDER --ginkgo.junit-report test-results.xml
  out=$?
fi

if  [[ $out != 0 ]]  ; then
  echo "::endgroup::"
  exit 1
fi
