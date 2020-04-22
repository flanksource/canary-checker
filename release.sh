#!/bin/bash
set -x
cd $GITHUB_WORKSPACE
GITHUB_USER=$(echo $GITHUB_REPOSITORY | cut -d/ -f1)
NAME=$(echo $GITHUB_REPOSITORY | cut -d/ -f2)
VERSION="v$GITHUB_REF built $(date)"

make setup linux darwin compress

github-release release -u $GITHUB_USER -r ${NAME} --tag $GITHUB_REF
github-release upload -R -u $GITHUB_USER -r ${NAME} --tag $GITHUB_REF -n ${NAME} -f .bin/${NAME}
github-release upload -R -u $GITHUB_USER -r ${NAME} --tag $GITHUB_REF -n ${NAME}_osx -f .bin/${NAME}_osx
