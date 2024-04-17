#!/bin/bash

set -e

if ! which mergestat  > /dev/null; then
    if $(uname -a | grep -q Darwin); then
    curl -L https://github.com/flanksource/askgit/releases/download/v0.61.0-flanksource.1/mergestat-macos-amd64.tar.gz  -o mergestat.tar.gz
    sudo tar xf mergestat.tar.gz -C /usr/local/bin/

    else
    curl -L https://github.com/flanksource/askgit/releases/download/v0.61.0-flanksource.1/mergestat-linux-amd64.tar.gz -o mergestat.tar.gz
    sudo tar xf mergestat.tar.gz -C  /usr/local/bin/
    fi
fi


kubectl create namespace canaries || true

# creating a GITHUB_TOKEN Secret
if [[ -z "${GH_TOKEN}" ]]; then
    printf "\nEnvironment variable for github token (GH_TOKEN) is missing!!!\n"
else
    printf "\nCreating secret from github token ending with '${GH_TOKEN:(-8)}'\n"
    kubectl create secret generic github-token --from-literal=GITHUB_TOKEN="${GH_TOKEN}" --namespace canaries
fi

helm repo add gitea-charts https://dl.gitea.io/charts
helm repo update
helm install gitea gitea-charts/gitea  -f fixtures/git/gitea.values --create-namespace --namespace gitea

sleep 300

kubectl get pods -n gitea
kubectl describe pods -n gitea

kubectl logs -n gitea deploy/gitea --all-containers || true

kubectl logs -n gitea statefulsets/gitea-postgresql --all-containers || true

sleep 100

kubectl get pods -n gitea
kubectl describe pods -n gitea

kubectl logs -n gitea deploy/gitea --all-containers || true

kubectl logs -n gitea statefulsets/gitea-postgresql --all-containers || true


kubectl port-forward  svc/gitea-http -n gitea 3001:3000 &
PID=$!

sleep 5

curl -vvv  -u gitea_admin:admin   -H "Content-Type: application/json"  http://localhost:3001/api/v1/user/repos  -d '{"name":"test_repo","auto_init":true}'

kill $PID

kubectl create secret generic gitea --from-literal=username=gitea_admin --from-literal=password=admin --from-literal=url=http://gitea-http.gitea.svc:3000/gitea_admin/test_repo.git --namespace canaries
