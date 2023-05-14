#!/bin/bash

set -e

# Install askgit
curl -L https://github.com/flanksource/askgit/releases/download/v0.4.8-flanksource/askgit-linux-amd64.tar.gz -o askgit.tar.gz
tar xf askgit.tar.gz
sudo mv askgit /usr/bin/askgit
sudo chmod +x /usr/bin/askgit
rm askgit.tar.gz

wget http://nz2.archive.ubuntu.com/ubuntu/pool/main/o/openssl/libssl1.1_1.1.1f-1ubuntu2.18_amd64.deb
sudo dpkg -i libssl1.1_1.1.1f-1ubuntu2.18_amd64.deb

#verification
which askgit
if ! askgit --help > /dev/null; then
    printf "`askgit --help` failed. Check the binary?"
    exit 1;
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
