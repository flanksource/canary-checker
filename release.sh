#!/bin/bash
set -x
cd $GITHUB_WORKSPACE
GITHUB_USER=$(echo $GITHUB_REPOSITORY | cut -d/ -f1)
NAME=$(echo $GITHUB_REPOSITORY | cut -d/ -f2)
TAG=$(echo $GITHUB_REF | sed 's|refs/tags/||')
VERSION="$TAG built $(date)"

make static linux darwin windows compress


github-release release -u $GITHUB_USER -r ${NAME} --tag $TAG || echo Release already created
github-release upload -R -u $GITHUB_USER -r ${NAME} --tag $TAG -n ${NAME} -f .bin/${NAME}
github-release upload -R -u $GITHUB_USER -r ${NAME} --tag $TAG -n ${NAME}_osx -f .bin/${NAME}_osx
github-release upload -R -u $GITHUB_USER -r ${NAME} --tag $TAG -n ${NAME}.exe -f .bin/${NAME}.exe

cd config

curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash

./kustomize edit set image flanksource/canary-checker:$TAG
./kustomize build  > release.yaml

github-release upload -R -u $GITHUB_USER -r ${NAME} --tag $TAG -n release.yaml -f release.yaml
