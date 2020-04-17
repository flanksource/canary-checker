#!/bin/bash

set -e

docker run -d --rm --name minio-canary-checker-e2e -it -p 9000:9000 -e MINIO_ACCESS_KEY=minio -e MINIO_SECRET_KEY=minio123 minio/minio:RELEASE.2019-10-12T01-39-57Z server /data

until curl --fail localhost:9000/minio/health/ready > /dev/null
do
  echo "Failed to connect to minio"
  sleep 2
done

echo "Minio started"

function cleanup {
  docker kill minio-canary-checker-e2e
}
trap cleanup EXIT

go test ./test

