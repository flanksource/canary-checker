#!/bin/bash
# ABOUTME: End-to-end test runner for canary-checker.
# ABOUTME: Provisions infrastructure, deploys test fixtures, and runs ginkgo tests in a Kind cluster.

set -e

export KUBECONFIG=~/.kube/config
export CLUSTER_NAME=kind-test
export PATH=$(pwd)/.bin:/usr/local/bin:$PATH
export ROOT=$(pwd)
export TEST_FOLDER=${TEST_FOLDER:-$1}
export TEST_BINARY=./test.test
SKIP_SETUP=${SKIP_SETUP:-false}
SKIP_CLUSTER=${SKIP_CLUSTER:-false}

if [[ "$1" == "" ]]; then
  echo "Usage ./test/e2e.sh TEST_FOLDER  [RunRegex] [--skip-setup] [--skip-cluster] [--skip-all] "
  exit 1
fi

if [[ "$*" == *"--skip-setup"* ]]; then
  SKIP_SETUP=true
fi
if [[ "$*" == *"--skip-cluster"* ]]; then
  SKIP_CLUSTER=true
fi
if [[ "$*" == *"--skip-all"* ]]; then
  SKIP_CLUSTER=true
  SKIP_SETUP=true
fi

echo "Testing $TEST_FOLDER with skip_setup=$SKIP_SETUP skip_cluster=$SKIP_CLUSTER"

if [[ "$SKIP_CLUSTER" != "true" ]] ; then
  echo "::group::Provisioning"
  # Kind cluster is provisioned by the CI workflow via helm/kind-action.
  if ! kind get clusters | grep -q $CLUSTER_NAME; then
    echo "Error: Kind cluster '$CLUSTER_NAME' not found. It should be provisioned by the workflow."
    exit 1
  fi
  echo "::endgroup::"

  bash $(pwd)/test/setup-infra.sh "$TEST_FOLDER"
fi


if [ "$SKIP_SETUP" != "true" ]; then
  echo "::group::Setting up"
  export GOOS=linux
  export GOARCH=amd64
  kubectl create ns canaries || true

  if [ -e $TEST_FOLDER/_setup.sh ]; then
    bash $TEST_FOLDER/_setup.sh || echo Setup failed, attempting tests anyway
  fi

  if [ -e $TEST_FOLDER/_setup.yaml ]; then
    kubectl apply -f $(pwd)/$TEST_FOLDER/_setup.yaml
  fi

  if [ -e $TEST_FOLDER/../_setup.yaml ]; then
    kubectl apply -f $(pwd)/$TEST_FOLDER/../_setup.yaml
  fi

  if [ -e $TEST_FOLDER/_post_setup.sh ]; then
    bash $TEST_FOLDER/_post_setup.sh || echo Post setup failed, attempting tests anyway
  fi

  if [ -e $TEST_FOLDER/main.go ]; then
    # Port-forward MinIO to the host for S3 setup
    kubectl port-forward -n minio svc/minio 9000:9000 &
    PF_PID=$!
    sleep 2
    export S3_ENDPOINT=http://localhost:9000
    cd $TEST_FOLDER
    go run main.go
    cd $ROOT
    kill $PF_PID 2>/dev/null || true
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
