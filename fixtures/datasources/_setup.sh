#!/bin/bash

set -e

restic version
# Initialize Restic Repo
# Do not fail if it already exists
RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.${DOMAIN}/restic-canary-checker init || true
#take some backup in restic
RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.${DOMAIN}/restic-canary-checker backup "$(pwd)"
#Sleep for 5 seconds for restic to be ready
sleep 5