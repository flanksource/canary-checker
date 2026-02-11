#!/bin/bash

set -e

kubectl create namespace canaries || true

helm repo add gitea-charts https://dl.gitea.io/charts
helm repo update
helm install gitea gitea-charts/gitea  -f fixtures/git/gitea.values --create-namespace --namespace gitea --wait

kubectl port-forward  svc/gitea-http -n gitea 3001:3000 &
PID=$!

sleep 5

curl -vvv  -u gitea_admin:admin   -H "Content-Type: application/json"  http://localhost:3001/api/v1/user/repos  -d '{"name":"test_repo","auto_init":true}'

kill $PID

kubectl create secret generic gitea --from-literal=username=gitea_admin --from-literal=password=admin --from-literal=url=http://gitea-http.gitea.svc:3000/gitea_admin/test_repo.git --namespace canaries
