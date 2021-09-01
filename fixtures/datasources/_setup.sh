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

kubectl port-forward -n podinfo-test svc/podinfo 33898:9898 &
kubectl port-forward -n platform-system svc/postgres 33432:5432 &
kubectl port-forward -n platform-system svc/redis 33379:6379 &
kubectl port-forward -n platform-system svc/mssql 33143:1433 &
kubectl port-forward -n platform-system svc/mongo 33017:27017 &
kubectl port-forward -n podinfo-test svc/podinfo 33999:9999 &
kubectl port-forward -n minio svc/minio 33000:9000 &
kubectl port-forward -n monitoring svc/prometheus-k8s 33090:9090 &
kubectl port-forward -n ldap svc/apacheds 33389:10389 &
kubectl port-forward -n ldap svc/apacheds 33636:10636 &

kubectl create secret generic aws-credentials --from-literal=AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID --from-literal=AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY -n podinfo-test -o yaml --dry-run | kubectl apply -n podinfo-test -f -

