#!/bin/bash

set -e

curl -sSLo /usr/local/bin/restic.bz2  https://github.com/restic/restic/releases/download/v0.12.1/restic_0.12.1_$(OS)_$(ARCH).bz2   && \
  bunzip2  /usr/local/bin/restic.bz2  && \
  chmod +x /usr/local/bin/restic


restic version
# Initialize Restic Repo
# Do not fail if it already exists
RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.${DOMAIN}/restic-canary-checker init || true
#take some backup in restic
RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.${DOMAIN}/restic-canary-checker backup "$(pwd)"
#Sleep for 5 seconds for restic to be ready
sleep 5
