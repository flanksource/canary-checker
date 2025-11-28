FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25-bookworm@sha256:e17419604b6d1f9bc245694425f0ec9b1b53685c80850900a376fb10cb0f70cb AS builder
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

FROM flanksource/base-image-canary-checker:0.5.20@sha256:2d29625a11aff42c2a5c222d7764807478abdd93549e4828043c909962b16fba
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
