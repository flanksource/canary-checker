FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.22-bookworm@sha256:f020456572fc292e9627b3fb435c6de5dfb8020fbcef1fd7b65dd092c0ac56bb AS builder
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

FROM --platform=${BUILDPLATFORM:-linux/amd64} flanksource/base-image-canary-checker:0.5.15
ARG TARGETARCH

WORKDIR /app

RUN mkdir /opt/database && groupadd --gid 1000 canary && \
  useradd canary --uid 1000 -g canary -m -d /var/lib/canary && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

USER canary:canary

ENV PATH="${PATH}:/var/lib/canary/bin/"

COPY --from=builder /app/.bin/canary-checker /app

RUN if [ "$TARGETARCH" = "amd64" ]; then \
    # This Docker image is built on an amd64 host
    /app/canary-checker go-offline; \
  fi

ENTRYPOINT ["/app/canary-checker"]
