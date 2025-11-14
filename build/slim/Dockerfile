FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25-bookworm@sha256:7419f544ffe9be4d7cbb5d2d2cef5bd6a77ec81996ae2ba15027656729445cc4 AS builder
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

FROM debian:bookworm-slim@sha256:936abff852736f951dab72d91a1b6337cf04217b2a77a5eaadc7c0f2f1ec1758 AS base
WORKDIR /app
ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive
ENV LC_ALL=en_US.UTF-8
ENV LANG=en_US.UTF-8

RUN apt-get update && \
  apt-get install -y \
    curl wget less \
    unzip  zip  wget gnupg2 bzip2  \
    locales locales-all tzdata \
    apt-transport-https ca-certificates lsb-release git python3-crcmod python3-openssl \
    jq yq \
    --no-install-recommends  && \
    locale-gen en_US.UTF-8 && \
    update-locale LANG=en_US.UTF-8 && \
    rm -Rf /var/lib/apt/lists/* && \
    apt-get clean

RUN apt-get update && apt-get upgrade -y && \
  rm -Rf /var/lib/apt/lists/* && \
  apt-get clean

RUN mkdir /opt/database && groupadd --gid 1000 canary && \
  useradd canary --uid 1000 -g canary -m -d /var/lib/canary && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

USER canary:canary

COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline

ENTRYPOINT ["/app/canary-checker"]
