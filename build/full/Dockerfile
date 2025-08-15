FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25-bookworm@sha256:74908ad827a5849c557eeca81d46263acf788ead606102d83466f499f83e35b1 AS builder
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

FROM flanksource/base-image-canary-checker:0.5.18
ARG TARGETARCH

WORKDIR /app

RUN mkdir /opt/database && groupadd --gid 1000 canary && \
  useradd canary --uid 1000 -g canary -m -d /var/lib/canary && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

USER canary:canary

ENV PATH="${PATH}:/var/lib/canary/bin/"

COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline

ENTRYPOINT ["/app/canary-checker"]
