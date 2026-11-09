FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.27.3-bookworm@sha256:d9b6a1beb9474adc2a1ce842573639b44b2a0dd49b6669f51d3821baf9ec37c9 AS builder
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
# Keep compiler scratch data out of exported layers; retain the module download layer.
RUN --mount=type=cache,target=/root/.cache/go-build,id=go-1.26.1-${TARGETOS}-${TARGETARCH} \
    OS=${TARGETOS} ARCH=${TARGETARCH} make build

FROM flanksource/base-image-canary-checker:0.9.2@sha256:2760572cf4bf0869d382aa9d9614487f9803a1ade8bd5b76f0ac6e787d4d3f8b
ARG TARGETARCH

WORKDIR /app

RUN apt-get update && \
  apt-get install -y \
    xz-utils libpq5 \
    --no-install-recommends  && \
    rm -Rf /var/lib/apt/lists/* && \
    apt-get clean

# restic for the restic check: deps cannot unpack restic's single-file .bz2 release assets, so pin the version and checksum here
ARG RESTIC_VERSION=0.19.1
RUN case "${TARGETARCH}" in \
      amd64) RESTIC_SHA256="f415415624dcc452f2a02b8c33641791a8c6d6d3b65bbb3543fcf9a25151585c" ;; \
      arm64) RESTIC_SHA256="a5f64aaab53d51e311fa3829124c5b703f2d14cf187d8640b6be3b2b49376465" ;; \
      *) echo "Unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac && \
    curl --fail --location --retry 5 -o /tmp/restic.bz2 https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_${TARGETARCH}.bz2 && \
    echo "${RESTIC_SHA256}  /tmp/restic.bz2" | sha256sum --check --strict && \
    bunzip2 -c /tmp/restic.bz2 > /usr/bin/restic && \
    chmod +x /usr/bin/restic && \
    rm /tmp/restic.bz2

RUN mkdir -p /opt/database && \
  (groupmod -n canary ubuntu 2>/dev/null || groupadd --gid 1000 canary) && \
  (usermod -l canary -d /var/lib/canary -m ubuntu 2>/dev/null || useradd canary --uid 1000 -g canary -m -d /var/lib/canary) && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

USER canary:canary

ENV PATH="${PATH}:/var/lib/canary/bin/"

COPY --from=builder /app/.bin/canary-checker /app

RUN --mount=type=secret,id=GITHUB_TOKEN,env=GITHUB_TOKEN \
    deps install postgrest@v14.4 --bin-dir /app/.bin

RUN --mount=type=secret,id=GITHUB_TOKEN,env=GITHUB_TOKEN \
    /app/canary-checker go-offline
RUN /app/.bin/postgrest --version

ENTRYPOINT ["/app/canary-checker"]
