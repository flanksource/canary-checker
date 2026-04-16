FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.26.1-bookworm@sha256:ab3d6955bbc813a0f3fdf220c1d817dd89c0b3f283777db8ece4a32fe7858edd AS builder
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

FROM flanksource/base-image:0.7.1@sha256:5e819e83199855d20ba5b988e8a6f369c6f4332fa6f07c184c99838d06a77cc8
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
