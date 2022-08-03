# docker container
## Check if the container is working

```bash
docker build -t canary-checker -f Dockerfile . # or something like this
docker run \
  -v `pwd`:'/tmp/' \
  -p=8080:8080 -p=5423:5432 -p=3000:3000 \
  -it \
  canary-checker serve \
    --db="postgres://postgres:postgres@host.docker.internal:5432/canary" # host.docker.internal points to host. Change to point to postgres instance \
    --httpPort=8080 \
    -c /tmp/test/aggregate-test/config/main.yaml # config for checkers \
    --maxStatusCheckCount 10
```
