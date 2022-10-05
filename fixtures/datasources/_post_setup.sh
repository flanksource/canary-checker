#!/bin/bash

echo "Running kubectl wait for elasticsearch"
kubectl -n default wait --for=condition=ready pod -l app=elasticsearch --timeout=5m

echo "Running kubectl wait for rabbitmq server"
kubectl -n default wait --for=condition=ready pod -l app.kubernetes.io/name=amqp-fixture --timeout=5m
