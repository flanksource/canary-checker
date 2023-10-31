#!/bin/bash

set -e

echo "::group::Prerequisites"
required_tools=("tr" "docker" "curl")
for tool in "${required_tools[@]}"; do
  if ! command -v $tool &>/dev/null; then
    echo "$tool is not installed. Please install it to run this script."
    exit 1
  fi
done
echo "All the required tools are installed."
echo "::endgroup::"

# https://cedwards.xyz/defer-for-shell/
DEFER=
defer() {
  DEFER="$*; ${DEFER}"
  trap "{ $DEFER }" EXIT
}

port=7676
container_name=webhook_postgres

## Summary
# - Fire up the canary checker HTTP server for webhook endpoint
# - Expect the checks to be created
# - Create resolved alert
# - Expect the checks to be deleted

echo "::group::Provisioning"
echo "Starting up postgres database"
docker run --rm -p 5433:5432 --name $container_name -e POSTGRES_PASSWORD=mysecretpassword -d postgres:14
defer docker container rm -f $container_name

echo "Starting canary-checker in the background"
go run main.go serve --httpPort=$port \
  --db-migrations \
  --disable-postgrest -vvv \
  --db='postgres://postgres:mysecretpassword@localhost:5433/postgres?sslmode=disable' \
  --maxStatusCheckCount=1 \
  fixtures/external/alertmanager.yaml &>/dev/null &
PROC_ID=$!
echo "Started canary checker with PID $PROC_ID"

timeout=30
echo Waiting for the server to come up. timeout=$timeout seconds
for ((i = 1; i <= $timeout; i++)); do
  if [ $(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$port/health") == "200" ]; then
    echo "Server healthy (HTTP 200 OK)."
    break
  fi

  [ $i -eq $timeout ] && echo "Timeout: Server didn't return HTTP 200." && exit 1
  sleep 1
done

# Not sure why killing PROC_ID doesn't kill the HTTP server.
# So had to get the process id this way
process_id=$(lsof -nti:$port)
echo "Running on port $process_id"
defer "kill -9 $process_id"
echo "::endgroup::"

echo "::group::Assertion"
echo Expect the check to be created by sync job
resp=$(docker exec $container_name psql "postgres://postgres:mysecretpassword@localhost:5432/postgres?sslmode=disable" -t -c "SELECT count(*) FROM checks WHERE name = 'my-webhook';" | tr -d '[:space:]')
if [ $resp -ne 1 ]; then
  echo "Expected one webhook check to be created but $resp were created"
  exit 1
fi

echo Attempt to call the webhook endpoint without the auth token
resp=$(curl -w "%{http_code}" -s -o /dev/null -X POST "localhost:$port/webhook/my-webhook")
if [ $resp -ne 401 ]; then
  echo "Expected 401, got $resp"
  exit 1
fi

echo Attempt to call the webhook endpoint with the auth token
resp=$(curl -w "%{http_code}" -s -o /dev/null -X POST "localhost:$port/webhook/my-webhook?token=webhook-auth-token")
if [ $resp -ne 200 ]; then
  echo "Expected 200, got $resp"
  exit 1
fi

echo Calling webhook endpoint with unresolved alert
curl -sL -o /dev/null -X POST -u 'admin@local:admin' --header "Content-Type: application/json" --data '{
  "version": "4",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "name": "first",
      "labels": {
        "severity": "critical",
        "alertName": "ServerDown",
        "location": "DataCenterA"
      },
      "annotations": {
        "summary": "Server in DataCenterA is down",
        "description": "This alert indicates that a server in DataCenterA is currently down."
      },
      "startsAt": "2023-10-30T08:00:00Z",
      "generatorURL": "http://example.com/generatorURL/serverdown",
      "fingerprint": "a1b2c3d4e5f6"
    },
    {
      "status": "resolved",
      "labels": {
        "severity": "warning",
        "alertName": "HighCPUUsage",
        "location": "DataCenterB"
      },
      "annotations": {
        "summary": "High CPU Usage in DataCenterB",
        "description": "This alert indicates that there was high CPU usage in DataCenterB, but it is now resolved."
      },
      "startsAt": "2023-10-30T09:00:00Z",
      "generatorURL": "http://example.com/generatorURL/highcpuusage", 
      "name": "second",
      "fingerprint": "x1y2z3w4v5"
    }
  ]
}' localhost:$port/webhook/my-webhook?token=webhook-auth-token

resp=$(docker exec $container_name psql 'postgres://postgres:mysecretpassword@localhost:5432/postgres?sslmode=disable' -t -c "SELECT count(*) FROM checks WHERE type = 'webhook' AND deleted_at IS NULL;" | tr -d '[:space:]')
if [ $resp -ne 3 ]; then
  echo "Expected 2 new checks to be created but $resp were found"
  exit 1
fi

echo Calling webhook endpoint with a resolved alert
curl -sL -o /dev/null -X POST -u 'admin@local:admin' --header "Content-Type: application/json" --data '{
  "version": "4",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "name": "first",
      "labels": {
        "severity": "critical",
        "alertName": "ServerDown",
        "location": "DataCenterA"
      },
      "annotations": {
        "summary": "Server in DataCenterA is down",
        "description": "This alert indicates that a server in DataCenterA is currently down."
      },
      "startsAt": "2023-10-30T08:00:00Z",
      "generatorURL": "http://example.com/generatorURL/serverdown",
      "fingerprint": "a1b2c3d4e5f6",
      "endsAt": "2023-10-30T09:15:00Z"
    }
  ]
}' localhost:$port/webhook/my-webhook?token=webhook-auth-token

resp=$(docker exec $container_name psql 'postgres://postgres:mysecretpassword@localhost:5432/postgres?sslmode=disable' -t -c "SELECT name FROM checks WHERE type = 'webhook' AND deleted_at IS NOT NULL;" | tr -d '[:space:]')
if [ "$resp" != 'firsta1b2c3d4e5f6' ]; then
  echo "Expected "firsta1b2c3d4e5f6" check to be deleted."
  exit 1
fi

echo "::endgroup::"
exit 0
