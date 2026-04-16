FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.26.1-bookworm@sha256:bd76f94e0ec7c7926e1dff7b90913f639608b8dd57315cd1c420624e577f95d3 AS builder
WORKDIR /app

ARG TARGETOS
ARG TARGETARCH

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=full

COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download

COPY ./ ./
RUN OS=${TARGETOS} ARCH=${TARGETARCH} make build

FROM flanksource/base-image-canary-checker:0.7.1@sha256:010f16e33084db3b94a4b741b5c895fa1274a3b5864bdc05b924940289c456cc
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

USER canary:canary

ENV PATH="${PATH}:/var/lib/canary/bin/"

COPY --from=builder /app/.bin/canary-checker /app

RUN deps install postgrest@v14.4 --bin-dir /app/.bin

RUN /app/canary-checker go-offline
RUN /app/.bin/postgrest --version

ENTRYPOINT ["/app/canary-checker"]
