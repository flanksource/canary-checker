#!/bin/bash

set -ex

export PLATFORM_CLI_VERSION=0.11.1-611-ge9c9629
export PLATFORM_CLI="./platform-cli -c test/config.yaml"

curl https://github.com/flanksource/platform-cli/releases/download/$PLATFORM_CLI_VERSION/platform-cli > platform-cli
chmod +x platform-cli

docker pull docker.io/library/busybox:1.30
docker tag docker.io/library/busybox:1.30 docker.io/flanksource/busybox:1.30

$PLATFORM_CLI ca generate --name ingress-ca --cert-path .certs/ingress-ca-crt.pem --private-key-path .certs/ingress-ca-key.pem --password foobar  --expiry 1
$PLATFORM_CLI ca generate --name sealed-secrets --cert-path .certs/sealed-secrets-crt.pem --private-key-path .certs/sealed-secrets-key.pem --password foobar  --expiry 1
$PLATFORM_CLI provision kind-cluster

$PLATFORM_CLI deploy stubs

go test ./test