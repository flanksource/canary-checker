#!/bin/bash

ns="-n $1"

kubectl apply -f fixtures/minimal/exec_pass.yaml $ns || exit 1

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
  kill "$PID" 2>/dev/null || true
  wait "$PID" 2>/dev/null || true
}

trap cleanup EXIT

deadline=$((SECONDS + 60))
ready=false
while (( SECONDS < deadline )); do
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "Port-forward exited before readiness"
    break
  fi
  status=$(kubectl get $ns canaries.canaries.flanksource.com exec-pass --request-timeout=5s -o jsonpath='{.status.status}')
  if [[ "$status" == "Passed" ]] &&
    curl --silent --show-error --fail --max-time 2 "http://localhost:$PORT/health" &&
    curl --silent --show-error --fail --max-time 2 "http://localhost:$PORT/db/checks"; then
    ready=true
    break
  fi
  sleep 2
done
echo "Status=$status"
if [[ "$ready" != "true" ]]; then
  echo "Timed out waiting for Passed status, health, and database readiness"
  kubectl get $ns canaries.canaries.flanksource.com exec-pass --request-timeout=5s -o yaml
  kubectl get pods $ns --request-timeout=5s
  kubectl logs $ns deploy/canary-checker --tail=100 --request-timeout=5s
  exit 1
fi

if ! curl -vv --fail --max-time 5 "http://localhost:$PORT/health"; then
  echo "Call to health failed"
  exit 1
fi

if ! curl -vv --fail --max-time 5 "http://localhost:$PORT/db/checks"; then
  # "we don't really care about the results as long as it is sucessful"
  echo "Call to postgrest failed"
  exit 1
fi
