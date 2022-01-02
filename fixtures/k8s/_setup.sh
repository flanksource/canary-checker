docker pull public.ecr.aws/docker/library/busybox:1.33.1
docker tag  public.ecr.aws/docker/library/busybox:1.33.1 ttl.sh/flanksource-busybox:1.33.1
docker tag  public.ecr.aws/docker/library/busybox:1.33.1 docker.io/flanksource/busybox:1.33.1

kubectl apply -k ../
