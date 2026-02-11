FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25-bookworm@sha256:ee420c17fa013f71eca6b35c3547b854c838d4f26056a34eb6171bba5bf8ece4 AS builder
WORKDIR /app

ARG TARGETOS
ARG TARGETARCH

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=slim

COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download

COPY ./ ./
RUN OS=${TARGETOS} ARCH=${TARGETARCH} make build

FROM ubuntu:noble@sha256:c35e29c9450151419d9448b0fd75374fec4fff364a27f176fb458d472dfc9e54 AS base
WORKDIR /app
ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive
ENV LC_ALL=en_US.UTF-8
ENV LANG=en_US.UTF-8

RUN apt-get update && \
  apt-get install -y \
    curl wget less \
    unzip zip gnupg2 bzip2  \
    locales locales-all tzdata \
    apt-transport-https ca-certificates lsb-release git python3-crcmod python3-openssl \
    jq yq \
    xz-utils libpq5 \
    --no-install-recommends  && \
    locale-gen en_US.UTF-8 && \
    update-locale LANG=en_US.UTF-8 && \
    rm -Rf /var/lib/apt/lists/* && \
    apt-get clean


RUN mkdir -p /opt/database && \
  (groupmod -n canary ubuntu 2>/dev/null || groupadd --gid 1000 canary) && \
  (usermod -l canary -d /var/lib/canary -m ubuntu 2>/dev/null || useradd canary --uid 1000 -g canary -m -d /var/lib/canary) && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

RUN curl -sL https://github.com/flanksource/deps/releases/latest/download/deps-linux-${TARGETARCH}.tar.gz -o deps-linux-${TARGETARCH}.tar.gz  && \
  tar -xzf deps-linux-${TARGETARCH}.tar.gz -C /usr/bin && \
  rm deps-linux-${TARGETARCH}.tar.gz && \
  chmod +x /usr/bin/deps

USER canary:canary

COPY --from=builder /app/.bin/canary-checker /app

RUN deps --no-progress install  postgrest@v14.4  --bin-dir /app/.bin

RUN /app/canary-checker go-offline

RUN /app/.bin/postgrest --version

ENTRYPOINT ["/app/canary-checker"]
