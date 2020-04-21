#!/bin/bash
set -x
cd $GITHUB_WORKSPACE
GITHUB_USER=$(echo $GITHUB_REPOSITORY | cut -d/ -f1)
NAME=$(echo $GITHUB_REPOSITORY | cut -d/ -f2)
TAG=$(git describe --tags --abbrev=0 --exact-match)
SNAPSHOT=false
if [[ "$TAG" == "" ]];  then
  SNAPSHOT=true
  TAG=$(git describe --tags --exclude "*-g*")
  if [[ "$TAG" == "" ]];  then
    TAG="0.1"
  fi
fi

VERSION="v$TAG built $(date)"

make setup linux darwin compress

if [[ "$SNAPSHOT" == "true" ]]; then
  echo Releasing pre-release
  github-release release -u $GITHUB_USER -r ${NAME} --tag $TAG --pre-release
else
  echo Releasing final release
  github-release release -u $GITHUB_USER -r ${NAME} --tag $TAG
fi

echo Uploading $NAME
github-release upload -R -u $GITHUB_USER -r ${NAME} --tag $TAG -n ${NAME} -f .bin/${NAME}
echo Uploading ${NAME}_osx
github-release upload -R -u $GITHUB_USER -r ${NAME} --tag $TAG -n ${NAME}_osx -f .bin/${NAME}_osx
