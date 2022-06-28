# Quick Start Guide
Before installing the Canary Checker, please ensure you have the following prerequisites installed on your Kubernetes Cluster. 




```bash
# install the operator
kubectl apply -f https://github.com/flanksource/canary-checker/releases/download/v0.13.5/release.yaml
# deploy a sample canary
kubectl apply -f https://raw.githubusercontent.com/flanksource/canary-checker/master/fixtures-crd/http_pass.yaml
# check the results of the canary
kubectl get canary
```

`sample output`

```
NAMESPACE         NAME   INTERVAL   STATUS   MESSAGE   UPTIME 1H      LATENCY 1H   LAST TRANSITIONED   LAST CHECK
platform-system   dns    30         Passed             0/2 (0%)                                        6s
platform-system   lan    30         Passed             12/12 (100%)   1033         139m                6s
platform-system   ldap   30         Passed             5/5 (100%)     323                              1s
platform-system   pod    120        Passed             1/2 (50%)      10904        45m                 24s
platform-system   s3     30         Passed             5/5 (100%)     1091         5m35s               5s
```

`http_pass.yaml`

```yaml
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: http-pass
spec:
  interval: 30
  http:
    - endpoint: https://httpstat.us/200
      thresholdMillis: 3000
      responseCodes: [201, 200, 301]
      responseContent: ""
      maxSSLExpiry: 7
```