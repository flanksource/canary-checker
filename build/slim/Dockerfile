FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.24-bookworm@sha256:29d97266c1d341b7424e2f5085440b74654ae0b61ecdba206bc12d6264844e21 AS builder
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

FROM debian:bookworm-slim@sha256:90522eeb7e5923ee2b871c639059537b30521272f10ca86fdbbbb2b75a8c40cd AS base
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
    --no-install-recommends  && \
    locale-gen en_US.UTF-8 && \
    update-locale LANG=en_US.UTF-8 && \
    rm -Rf /var/lib/apt/lists/* && \
    apt-get clean

RUN apt-get update && apt-get upgrade -y && \
  rm -Rf /var/lib/apt/lists/* && \
  apt-get clean


# stern, jq, yq
RUN curl -sLS https://get.arkade.dev | sh && \
  arkade get jq yq  --path /usr/bin && \
  chmod +x /usr/bin/jq /usr/bin/yq && \
  rm -Rf /usr/bin/arkade

RUN mkdir /opt/database && groupadd --gid 1000 canary && \
  useradd canary --uid 1000 -g canary -m -d /var/lib/canary && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

USER canary:canary

COPY --from=builder /app/.bin/canary-checker /app

RUN if [ "$TARGETARCH" = "amd64" ]; then \
  # This Docker image is built on an amd64 host
    /app/canary-checker go-offline; \
  fi

ENTRYPOINT ["/app/canary-checker"]
