#!/bin/bash

set -e

if ! which mergestat  > /dev/null; then
    if $(uname -a | grep -q Darwin); then
    curl -L https://github.com/flanksource/askgit/releases/download/v0.61.0-flanksource.1/mergestat-macos-amd64.tar.gz  -o mergestat.tar.gz
    sudo tar xf mergestat.tar.gz -C /usr/local/bin/

    else
    curl -L https://github.com/flanksource/askgit/releases/download/v0.61.0-flanksource.1/mergestat-linux-amd64.tar.gz -o mergestat.tar.gz
    sudo tar xf mergestat.tar.gz -C  /usr/local/bin/
    fi
fi

# creating a GITHUB_TOKEN Secret
if [[ -z "${GH_TOKEN}" ]]; then
    printf "\nEnvironment variable for github token (GH_TOKEN) is missing!!!\n"
    exit 1;
else
    printf "\nCreating secret from github token ending with '${GH_TOKEN:(-8)}'\n"
fi

kubectl create secret generic github-token --from-literal=GITHUB_TOKEN="${GH_TOKEN}" --namespace default
kubectl get secret github-token -o yaml --namespace default
