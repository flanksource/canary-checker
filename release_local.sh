#!/bin/bash
set -x

# RELEASE_FROM_LOCAL must be true for release from localhost
if [[ "$RELEASE_FROM_LOCAL" == "" ]]; then
  echo Skipping release, RELEASE_FROM_LOCAL is not set
  exit 0
fi
NAME=$(basename $(git remote get-url origin | sed 's/\.git//'))
GITHUB_USER=$(basename $(dirname $(git remote get-url origin | sed 's/\.git//')))
GITHUB_USER=${GITHUB_USER##*:}
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

make setup linux darwin

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

echo Building docker image

docker build --build-arg NAME=$NAME -t $GITHUB_USER/$NAME:$TAG -f build/Dockerfile .

echo Pushing docker image
docker login --username $DOCKER_LOGIN --password $DOCKER_PASS
docker push $GITHUB_USER/$NAME:$TAG

