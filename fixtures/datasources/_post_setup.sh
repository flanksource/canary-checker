#!/bin/bash

echo "Running kubectl wait for elasticsearch"
kubectl -n default wait --for=condition=ready pod -l app=elasticsearch --timeout=5m
