docker pull  public.ecr.aws/docker/library/busybox:1.31.0
docker tag  public.ecr.aws/docker/library/busybox:1.31.0 ttl.sh/flanksource-busybox:1.30
docker tag  public.ecr.aws/docker/library/busybox:1.31.0 docker.io/flanksource/busybox:1.30

kubectl apply -k ../
