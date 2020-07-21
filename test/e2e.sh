#!/bin/bash

set -ex

export PLATFORM_CLI_VERSION=v0.17.9
export PLATFORM_CLI="./platform-cli -c test/config.yaml"
export KUBECONFIG=~/.kube/config
export DOCKER_API_VERSION=1.39


if which karina 2>&1 > /dev/null; then
  PLATFORM_CLI="karina -c test/config.yaml"
else
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
fi

mkdir -p .bin

$PLATFORM_CLI ca generate --name root-ca --cert-path .certs/root-ca.crt --private-key-path .certs/root-ca.key --password foobar  --expiry 1
$PLATFORM_CLI ca generate --name ingress-ca --cert-path .certs/ingress-ca.crt --private-key-path .certs/ingress-ca.key --password foobar  --expiry 1
$PLATFORM_CLI provision kind-cluster


$PLATFORM_CLI deploy phases --base --stubs --calico
$PLATFORM_CLI test stubs --wait=480
$PLATFORM_CLI apply test/setup.yml

export DOCKER_USERNAME=test
export DOCKER_PASSWORD=password

wget -q https://github.com/atkrad/wait4x/releases/download/v0.3.0/wait4x-linux-amd64  -O ./wait4x
chmod +x ./wait4x

./wait4x tcp 127.0.0.1:30636 -t 120s -i 5s || true
./wait4x tcp 127.0.0.1:30389 || true
./wait4x tcp 127.0.0.1:32432 || true

make static
cd test
go test ./... -v -c
# ICMP requires privelages so we run the tests with sudo
sudo DOCKER_API_VERSION=1.39 ./test.test  -test.v
