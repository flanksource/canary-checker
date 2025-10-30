#!/bin/bash

ns="-n $1"

kubectl apply -f fixtures/minimal/exec_pass.yaml $ns

kubectl get pods --all-namespaces

kubectl logs -n canary-checker deploy/canary-checker

function get_unused_port() {
  for port in $(seq 4444 65000);
  do
    echo -ne "\035" | telnet 127.0.0.1 $port > /dev/null 2>&1;
    [ $? -eq 1 ] && echo "$port" && break;
  done
}


PORT=$(get_unused_port)
kubectl port-forward $ns  svc/canary-checker $PORT:8080 &
PID=$!
function cleanup {
  echo "Cleaning up..."
  kill $PID
}

trap cleanup EXIT

sleep 60

status=$(kubectl get  $ns canaries.canaries.flanksource.com exec-pass -o yaml | yq .status.status)
echo "Status=$status"
if [[ $status != "Passed" ]]; then
  exit 1
fi

if ! curl -vv --fail "http://localhost:$PORT/health"; then
  # "we don't really care about the results as long as it is sucessful"
  echo "Call to health failed"
fi

if ! curl -vv --fail "http://localhost:$PORT/db/"; then
  # "we don't really care about the results as long as it is sucessful"
  echo "Call to /db/ failed"
fi

if ! curl -vv --fail "http://localhost:$PORT/db/canaries"; then
  # "we don't really care about the results as long as it is sucessful"
  echo "Call to canaries failed"
fi

if ! curl -vv --fail "http://localhost:$PORT/db/checks"; then
  # "we don't really care about the results as long as it is sucessful"
  echo "Call to postgrest failed"
  exit 1
fi
