#!/bin/bash

set -e

export KARINA="karina -c test/config.yaml"
export KUBECONFIG=~/.kube/config
export DOCKER_API_VERSION=1.39
export CLUSTER_NAME=kind-test
export PATH=.bin/$PATH

SKIP_K8S_SETUP=${SKIP_K8S_SETUP:-false}
WAIT=${WAIT:-true}
RESTIC=${RESTIC:-true}
UI=${UI:-true}


if ! $SKIP_K8S_SETUP ; then
  echo "$(kubectl config current-context) != kind-$CLUSTER_NAME"
  if [[ "$(kubectl config current-context)" != "kind-$CLUSTER_NAME" ]] ; then
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
  fi

  kubectl config use-context kind-$CLUSTER_NAME

  export PATH=$(pwd)/.bin:$PATH

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

echo "::group::Testing"
export DOCKER_USERNAME=test
export DOCKER_PASSWORD=password


if $WAIT ; then
  wait4x tcp 127.0.0.1:30636 -t 120s -i 5s || true
  wait4x tcp 127.0.0.1:30389 || true
  wait4x tcp 127.0.0.1:32432 || true
  wait4x tcp 127.0.0.1:32004 || true
  wait4x tcp 127.0.0.1:32010 || true
  wait4x tcp 127.0.0.1:32018 || true
  wait4x tcp 127.0.0.1:32015 || true
fi

if $RESTIC ; then
  #Verify
  restic version
  # Initialize Restic Repo
  # Do not fail if it already exists
  RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker init || true
  #take some backup in restic
  RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker backup $(pwd)
fi;

if $UI ; then
  make ui
fi


kubectl apply -R -f test/nested-canaries/
kubectl create secret generic aws-credentials --from-literal=AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID --from-literal=AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY -n podinfo-test -o yaml --dry-run | kubectl apply -n podinfo-test -f -

cd test
# first compile the test binary
go test ./... -v -c
USER=$(whoami)
sudo DOCKER_API_VERSION=1.39 --preserve-env=KUBECONFIG,TEST_FOLDER ./test.test  -test.v  2>&1 | tee test.out
sudo chown $USER test.out
cat test.out | go-junit-report > test-results.xml

echo "::endgroup::"
