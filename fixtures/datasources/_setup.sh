WAIT=${WAIT:-true}
RESTIC=${RESTIC:-true}

if $RESTIC ; then
  restic version
  # Initialize Restic Repo
  # Do not fail if it already exists
  RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker init || true
  #take some backup in restic
  RESTIC_PASSWORD="S0m3p@sswd" AWS_ACCESS_KEY_ID="minio" AWS_SECRET_ACCESS_KEY="minio123" restic --cacert .certs/ingress-ca.crt -r s3:https://minio.127.0.0.1.nip.io/restic-canary-checker backup $(pwd)

  echo "::group::Deploying Stubs"
  $KARINA deploy apacheds
  echo "::endgroup::"
  echo "::deploy monitoring::"
  $KARINA deploy monitoring
  echo "::endgroup::"
  #karina test stubs --wait=480 -v 5
  echo "::group::Setting up test environment"
  kubectl -n ldap delete svc apacheds
  echo "::endgroup::"
fi
if $WAIT ; then
  wait4x tcp 127.0.0.1:30636 -t 120s -i 5s || true
  wait4x tcp 127.0.0.1:30389 || true
  wait4x tcp 127.0.0.1:32432 || true
  wait4x tcp 127.0.0.1:32004 || true
  wait4x tcp 127.0.0.1:32010 || true
  wait4x tcp 127.0.0.1:32018 || true
  wait4x tcp 127.0.0.1:32015 || true
fi

kubectl create secret generic aws-credentials --from-literal=AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID --from-literal=AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY -n podinfo-test -o yaml --dry-run | kubectl apply -n podinfo-test -f -

