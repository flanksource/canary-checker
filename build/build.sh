#!/bin/bash

# Create Docker Image from root of Repo
docker_image_name="canary-checker:v1"
docker build -t $docker_image_name --build-arg VERSION="$VERSION" -f build/Dockerfile .

# RUN docker container
command_to_run="run -c http_multiple.yaml -v 2"
docker run --sysctl net.ipv4.ping_group_range="0   2147483647" -it $docker_image_name $command_to_run
