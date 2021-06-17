#!/bin/bash

set -ex

export PLATFORM_CLI_VERSION=v0.28.0
export PLATFORM_CLI="./karina -c test/config.yaml"
export KUBECONFIG=~/.kube/config
export DOCKER_API_VERSION=1.39
export CLUSTER_NAME=test


if which karina 2>&1 > /dev/null; then
  PLATFORM_CLI="karina -c test/config.yaml"
else
  if [[ "$OSTYPE" == "linux-gnu" ]]; then
    wget -q https://github.com/flanksource/karina/releases/download/$PLATFORM_CLI_VERSION/karina
    chmod +x karina
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    wget -q https://github.com/flanksource/karina/releases/download/$PLATFORM_CLI_VERSION/karina_osx
    cp karina_osx karina
    chmod +x karina
  else
    echo "OS $OSTYPE not supported"
    exit 1
  fi
fi

echo "$(kubectl config current-context) != kind-$CLUSTER_NAME"

if [[ "$(kubectl config current-context)" != "kind-$CLUSTER_NAME" ]] ; then
  echo "::group::Provisioning"
  if [[ ! -e .certs/root-ca.key ]]; then
    $PLATFORM_CLI ca generate --name root-ca --cert-path .certs/root-ca.crt --private-key-path .certs/root-ca.key --password foobar  --expiry 1
    $PLATFORM_CLI ca generate --name ingress-ca --cert-path .certs/ingress-ca.crt --private-key-path .certs/ingress-ca.key --password foobar  --expiry 1
    $PLATFORM_CLI ca generate --name sealed-secrets --cert-path .certs/sealed-secrets-crt.pem --private-key-path .certs/sealed-secrets-key.pem --password foobar  --expiry 1
  fi
  if $PLATFORM_CLI provision kind-cluster --trace -vv ; then
    echo "::endgroup::"
  else
    echo "::endgroup::"
    exit 1
  fi
fi


echo "::group::Deploying Base"
$PLATFORM_CLI deploy phases --crds --calico --base --stubs --minio --ingress
echo "::endgroup::"
#$PLATFORM_CLI test stubs --wait=480 -v 5
echo "::group::Setting up test environment"
$PLATFORM_CLI apply test/setup.yml
echo "::endgroup::"

echo "::group::Testing"
export DOCKER_USERNAME=test
export DOCKER_PASSWORD=password

wget -q https://github.com/atkrad/wait4x/releases/download/v0.3.0/wait4x-linux-amd64  -O ./wait4x
chmod +x ./wait4x

./wait4x tcp 127.0.0.1:30636 -t 120s -i 5s || true
./wait4x tcp 127.0.0.1:30389 || true
./wait4x tcp 127.0.0.1:32432 || true
./wait4x tcp 127.0.0.1:32004 || true
./wait4x tcp 127.0.0.1:32010 || true

#Install jmeter
curl -L https://mirrors.estointernet.in/apache//jmeter/binaries/apache-jmeter-5.4.1.tgz -o apache-jmeter-5.4.1.tgz && \
sudo tar xf apache-jmeter-5.4.1.tgz -C / && \
rm apache-jmeter-5.4.1.tgz && \
sudo apt-get update -y
sudo apt-get install -y openjdk-11-jre-headless
sudo ln -s /apache-jmeter-5.4.1/bin/jmeter /usr/local/bin/jmeter
#verification
jmeter -v

#Install Restic
sudo apt-get install -y curl
sudo curl -L https://github.com/restic/restic/releases/download/v0.12.0/restic_0.12.0_linux_amd64.bz2 -o /usr/bin/restic.bz2
sudo bunzip2  /usr/bin/restic.bz2
sudo chmod +x /usr/bin/restic
rm -rf /usr/bin/restic.bz2

#Verify
restic version
# Initialize Restic Repo
RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker init
#take some backup in restic
RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker backup /home/runner/work/canary-checker/canary-checker
make vue-dist
cd test
go test ./... -v -c
# ICMP requires privelages so we run the tests with sudo
sudo DOCKER_API_VERSION=1.39 --preserve-env=KUBECONFIG ./test.test  -test.v
echo "::endgroup::"