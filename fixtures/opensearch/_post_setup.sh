#!/bin/bash

echo "Running kubectl wait for opensearch"
kubectl -n canaries wait --for=condition=ready pod -l app=opensearch --timeout=5m

echo "Fetching elastic search health";
curl -s "http://opensearch.canaries.svc.cluster.local:9200/_cluster/health" -H 'Content-Type: application/json';
curl -s "http://opensearch.canaries.svc.cluster.local:9200/_cluster/allocation/explain" -H 'Content-Type: application/json';

kubectl get pods --all-namespaces

echo "Fetching populate-db logs from opensearch pod";
kubectl logs -n canaries -l app=opensearch
