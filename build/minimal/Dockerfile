FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.27.3-bookworm@sha256:d9b6a1beb9474adc2a1ce842573639b44b2a0dd49b6669f51d3821baf9ec37c9 AS builder
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
# Keep compiler scratch data out of exported layers; retain the module download layer.
RUN --mount=type=cache,target=/root/.cache/go-build,id=go-1.26.1-${TARGETOS}-${TARGETARCH} \
    OS=${TARGETOS} ARCH=${TARGETARCH} make build

FROM flanksource/base-image:0.9.0@sha256:17339ffc61a37baf58dd0dd85d8b5e46357c3dcb451f1bbc2fc2bd99576eb70f
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

RUN --mount=type=secret,id=GITHUB_TOKEN,env=GITHUB_TOKEN \
    deps install postgrest@v14.4 --bin-dir /app/.bin

RUN --mount=type=secret,id=GITHUB_TOKEN,env=GITHUB_TOKEN \
    /app/canary-checker go-offline
RUN /app/.bin/postgrest --version

ENTRYPOINT ["/app/canary-checker"]
