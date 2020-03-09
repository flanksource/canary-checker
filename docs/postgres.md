# Check postgres connectivity and query results

This check will try to connect to a specified Postgresql database,
run a query against it and verify the results.

```yaml
postgres:
  - connection: "user=postgres password=mysecretpassword host=192.168.0.103 port=15432 dbname=postgres sslmode=disable"
    query:  "SELECT 1"
    results: 1
```
