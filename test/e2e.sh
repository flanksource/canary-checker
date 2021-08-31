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


if ! $KARINA_SETUP ; then
  echo "::group::Provisioning"
  if [[ ! -e .certs/root-ca.key ]]; then
    $KARINA ca generate --name root-ca --cert-path .certs/root-ca.crt --private-key-path .certs/root-ca.key --password foobar  --expiry 1
    $KARINA ca generate --name ingress-ca --cert-path .certs/ingress-ca.crt --private-key-path .certs/ingress-ca.key --password foobar  --expiry 1
    $KARINA ca generate --name sealed-secrets --cert-path .certs/sealed-secrets-crt.pem --private-key-path .certs/sealed-secrets-key.pem --password foobar  --expiry 1
  fi

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
  echo "::endgroup::"
fi

echo "::group::Waiting for environment"

export DOCKER_USERNAME=test
export DOCKER_PASSWORD=password

kubectl port-forward -n podinfo-test svc/podinfo 33898:9898 &
kubectl port-forward -n platform-system svc/postgres 33432:5432 &
kubectl port-forward -n platform-system svc/redis 33379:6379 &
kubectl port-forward -n platform-system svc/mssql 33143:1433 &
kubectl port-forward -n platform-system svc/mongo 33017:27017 &
kubectl port-forward -n podinfo-test svc/podinfo 33999:9999 &
kubectl port-forward -n minio svc/minio 33000:9000 &
kubectl port-forward -n monitoring svc/prometheus-k8s 33090:9090 &
kubectl port-forward -n ldap svc/apacheds 33389:10389 &
kubectl port-forward -n ldap svc/apacheds 33636:10636 &


if $RESTIC ; then
  #Verify
  restic version
  # Initialize Restic Repo
  # Do not fail if it already exists
  RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker init || true
  #take some backup in restic
  RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker backup $(pwd)
fi;
echo "::endgroup::"

if $UI ; then
  echo "::group::Building UI"
  make ui
  echo "::endgroup::"
fi


kubectl apply -R -f test/nested-canaries/
kubectl create secret generic aws-credentials --from-literal=AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID --from-literal=AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY -n podinfo-test -o yaml --dry-run | kubectl apply -n podinfo-test -f -

echo "::group::Testing"
cd test
# first compile the test binary
go test ./... -v -c
USER=$(whoami)
sudo DOCKER_API_VERSION=1.39 --preserve-env=KUBECONFIG,TEST_FOLDER ./test.test  -test.v  2>&1 | tee test.out
sudo chown $USER test.out
cat test.out | go-junit-report > test-results.xml

echo "::endgroup::"
