FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25-bookworm@sha256:2f768d462dbffbb0f0b3a5171009f162945b086f326e0b2a8fd5d29c3219ff14 AS builder
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

FROM flanksource/base-image:0.6.0@sha256:6cae0a4bbba7e7e16674a55751c8161c11d5ebdd23f596f93e669f835ee1e034
ARG TARGETARCH

WORKDIR /app

RUN mkdir /opt/database && groupadd --gid 1000 canary && \
  useradd canary --uid 1000 -g canary -m -d /var/lib/canary && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

USER canary:canary

COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline

ENTRYPOINT ["/app/canary-checker"]
