#!/bin/bash

set -e

# Install askgit
curl -L https://github.com/flanksource/askgit/releases/download/v0.4.8-flanksource/askgit-linux-amd64.tar.gz -o askgit.tar.gz
tar xf askgit.tar.gz
sudo mv askgit /usr/bin/askgit
sudo chmod +x /usr/bin/askgit
rm askgit.tar.gz

#verification
which askgit
askgit --help

# creating a GITHUB_TOKEN Secret
kubectl create secret generic github-token --from-literal=GITHUB_TOKEN="${GH_TOKEN}" --namespace default

kubectl get secret github-token -o yaml --namespace default