#!/bin/bash

# The script tests the push subcommand as well as postgres convectivity for canary-checker.

set -e

export KUBECONFIG=~/.kube/config
export KARINA="karina -c $(pwd)/test/karina.yaml"
export DOCKER_API_VERSION=1.39
export CLUSTER_NAME='kind-test'
export PATH=$(pwd)/.bin:$PATH
export ROOT=$(pwd)

echo "::group::Provisioning"
if [[ ! -e .certs/root-ca.key ]]; then
$KARINA ca generate --name root-ca --cert-path .certs/root-ca.crt --private-key-path .certs/root-ca.key --password foobar  --expiry 1
$KARINA ca generate --name ingress-ca --cert-path .certs/ingress-ca.crt --private-key-path .certs/ingress-ca.key --password foobar  --expiry 1
$KARINA ca generate --name sealed-secrets --cert-path .certs/sealed-secrets-crt.pem --private-key-path .certs/sealed-secrets-key.pem --password foobar  --expiry 1
fi

## starting the postgres as docker container
docker run --rm -p 5433:5432  --name some-postgres -e POSTGRES_PASSWORD=mysecretpassword -d postgres:14.1

curl https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh -o wait-for-it.sh;
chmod +x wait-for-it.sh;

cat $KUBECONFIG
kubectl config use-context kind-$CLUSTER_NAME

echo "Waiting for server to accept connections"
./wait-for-it.sh 0.0.0.0:5433 --timeout=120;

echo "::group::Deploying Base"
## applying CRD and a sample fixture for the operator
kubectl apply -f config/deploy/Canary.yml
kubectl apply -f config/deploy/Topology.yml
kubectl apply -f config/deploy/Component.yml

## FIXME: kubectl wait for condition on CRD
# kubectl wait --for condition=established --timeout=60s crd/canaries.canaries.flanksource.com
sleep 10
echo "::endgroup::"


echo "::group::Operator"
## starting operator in background
go run main.go operator --db-migrations -vvv --db="postgres://postgres:mysecretpassword@localhost:5433/postgres?sslmode=disable"  &
PROC_ID=$!

echo "Started operator with PID $PROC_ID"

## sleeping for a bit to let the operator start and statuses to be present
sleep 120


./wait-for-it.sh 0.0.0.0:8080 --timeout=120;

echo "Server is ready now"

i=0
while [ $i -lt 5 ]
do
    go run main.go push http://0.0.0.0:8080 --name abc$i --description a --type junit  --duration 10 --message "10 of 10 passed"
    i=$((i+1))
done


CANARY_COUNT=$(kubectl get canaries.canaries.flanksource.com -A --no-headers | wc -l)
CANARY_COUNT=$(echo "$CANARY_COUNT" | xargs)
STATUS_COUNT_POSTGRES=$(curl -s http://0.0.0.0:8080/api/summary | jq ".checks_summary | length")


echo "::group::Dry Running Fixtures"

for fixture in minimal datasources k8s git ldap opensearch prometheus external elasticsearch aws azure; do
    for f in $(find ./fixtures/$fixture -name "*.yaml" ! -name "kustomization.yaml" ! -name "_*" ); do
        kubectl apply -f $f --dry-run=server
    done
done
echo "::endgroup::"


echo "Canary count: ${CANARY_COUNT}"
echo "Postgres count: ${STATUS_COUNT_POSTGRES}"


if [ "${CANARY_COUNT}" -gt 0 ]; then
    echo "Number of canaries is greater than 0: ${CANARY_COUNT}"
    exit 1
fi

if [ "${STATUS_COUNT_POSTGRES}" -ge 4 ]; then
    sudo kill -9 $PROC_ID || :
    echo "::endgroup::"
    exit 0
else
    echo "expected statuses length to be greater than 2 but got ${STATUS_COUNT_POSTGRES}"
    sudo kill -9 $PROC_ID || :
    echo "::endgroup::"
    exit 1
fi
