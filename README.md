# canary-checker

1. To build the docker image: `make image`

2. To run locally built image: `docker run --sysctl net.ipv4.ping_group_range="0   2147483647" -it canary-checker OPTIONS`

3. To run image: `docker run --sysctl net.ipv4.ping_group_range="0   2147483647" -it flanksource/canary-checker OPTIONS`
