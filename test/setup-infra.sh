#!/bin/bash
# ABOUTME: Deploys test infrastructure (ingress-nginx, cert-manager, MinIO) into a Kind cluster.
# ABOUTME: Called by e2e.sh; conditionally installs components based on the test suite path.

set -e

TEST_FOLDER="${1:-}"

echo "::group::Deploying infrastructure for $TEST_FOLDER"

# ingress-nginx: needed by k8s suite (ingress accessibility check)
if [[ "$TEST_FOLDER" == *"k8s"* ]]; then
  echo "Installing ingress-nginx..."
  kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.1/deploy/static/provider/kind/deploy.yaml
  echo "Waiting for ingress-nginx controller to be ready..."
  kubectl -n ingress-nginx wait --for=condition=ready pod -l app.kubernetes.io/component=controller --timeout=5m
fi

# cert-manager: needed by k8s suite (Certificate resource check)
if [[ "$TEST_FOLDER" == *"k8s"* ]]; then
  echo "Installing cert-manager..."
  kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.17.2/cert-manager.yaml
  echo "Waiting for cert-manager to be ready..."
  kubectl -n cert-manager wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager --timeout=5m
fi

# MinIO: needed by datasources suite (S3 bucket checks)
if [[ "$TEST_FOLDER" == *"datasources"* ]]; then
  echo "Installing MinIO..."
  kubectl apply -f "$(dirname "$0")/minio.yaml"
  echo "Waiting for MinIO to be ready..."
  kubectl -n minio wait --for=condition=ready pod -l app=minio --timeout=5m
fi

echo "::endgroup::"
