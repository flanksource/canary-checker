FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25-bookworm@sha256:ee420c17fa013f71eca6b35c3547b854c838d4f26056a34eb6171bba5bf8ece4 AS builder
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

FROM debian:bookworm-slim@sha256:12c396bd585df7ec21d5679bb6a83d4878bc4415ce926c9e5ea6426d23c60bdc AS base
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
# Install arkade version 0.11.36
RUN curl -sLS https://github.com/alexellis/arkade/releases/download/0.11.36/arkade -o /usr/local/bin/arkade && \
  echo "90f91dfba8fdc2ced936a52e40e6c8090e2e51e7ab14b81d27e1a9c2dadd9e1e  /usr/local/bin/arkade" | sha256sum -c - && \
  chmod +x /usr/local/bin/arkade && \
  arkade get jq yq --path /usr/bin && \
  chmod +x /usr/bin/jq /usr/bin/yq && \
  rm -Rf /usr/local/bin/arkade

RUN mkdir /opt/database && groupadd --gid 1000 canary && \
  useradd canary --uid 1000 -g canary -m -d /var/lib/canary && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

USER canary:canary

COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline

ENTRYPOINT ["/app/canary-checker"]
