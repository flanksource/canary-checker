FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.26.2-bookworm@sha256:47ce5636e9936b2c5cbf708925578ef386b4f8872aec74a67bd13a627d242b19 AS builder
WORKDIR /app

ARG TARGETOS
ARG TARGETARCH

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=minimal

COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download

COPY ./ ./
RUN OS=${TARGETOS} ARCH=${TARGETARCH} make build

FROM flanksource/base-image:0.8.0@sha256:b0cf87d5baa8fcfaad1294a09168f4215b1d2e98cec9ff93a49e48f2dd8fb0a6
ARG TARGETARCH

WORKDIR /app


RUN apt-get update && \
  apt-get install -y \
    xz-utils libpq5 \
    --no-install-recommends  && \
    rm -Rf /var/lib/apt/lists/* && \
    apt-get clean

RUN mkdir -p /opt/database && \
  (groupmod -n canary ubuntu 2>/dev/null || groupadd --gid 1000 canary) && \
  (usermod -l canary -d /var/lib/canary -m ubuntu 2>/dev/null || useradd canary --uid 1000 -g canary -m -d /var/lib/canary) && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

RUN apt-get update && apt-get install -y xz-utils wget libpq5 file && apt-get clean && rm -rf /var/lib/apt/lists/*
USER canary:canary

COPY --from=builder /app/.bin/canary-checker /app

RUN deps install postgrest@v14.4 --bin-dir /app/.bin

RUN /app/canary-checker go-offline
RUN /app/.bin/postgrest --version

ENTRYPOINT ["/app/canary-checker"]
