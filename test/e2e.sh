#!/bin/bash

set -ex

export PLATFORM_CLI_VERSION=0.11.1-623-gff09e24
export PLATFORM_CLI="./platform-cli -c test/config.yaml"

if [[ "$OSTYPE" == "linux-gnu" ]]; then
  wget -q https://github.com/flanksource/platform-cli/releases/download/$PLATFORM_CLI_VERSION/platform-cli
  chmod +x platform-cli
elif [[ "$OSTYPE" == "darwin"* ]]; then
  wget -q https://github.com/flanksource/platform-cli/releases/download/$PLATFORM_CLI_VERSION/platform-cli_osx
  cp platform-cli_osx platform-cli
  chmod +x platform-cli
else
  echo "OS $OSTYPE not supported"
  exit 1
fi

mkdir -p .bin

docker pull docker.io/library/busybox:1.30
docker tag docker.io/library/busybox:1.30 ttl.sh/flanksource-busybox:1.30
docker tag docker.io/library/busybox:1.30 docker.io/flanksource/busybox:1.30

$PLATFORM_CLI ca generate --name root-ca --cert-path .certs/root-ca.crt --private-key-path .certs/root-ca.key --password foobar  --expiry 1
$PLATFORM_CLI ca generate --name ingress-ca --cert-path .certs/ingress-ca.crt --private-key-path .certs/ingress-ca.key --password foobar  --expiry 1
$PLATFORM_CLI provision kind-cluster

$PLATFORM_CLI deploy calico
kubectl -n kube-system set env daemonset/calico-node FELIX_IGNORELOOSERPF=true

$PLATFORM_CLI deploy base
$PLATFORM_CLI deploy stubs

until curl --fail -k https://minio.127.0.0.1.nip.io/minio/health/ready > /dev/null
do
  echo "Failed to connect to minio"
  sleep 2
done

export DOCKER_USERNAME=test
export DOCKER_PASSWORD=password

go test ./test