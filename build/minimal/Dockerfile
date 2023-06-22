FROM golang:1.20@sha256:690e4135bf2a4571a572bfd5ddfa806b1cb9c3dea0446ebadaf32bc2ea09d4f9 AS builder
WORKDIR /app

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=minimal
COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download
COPY ./ ./
RUN go version
RUN make build

FROM ubuntu 
WORKDIR /app
RUN apt-get update && \
  apt-get install -y curl unzip ca-certificates jq wget gnupg2 bzip2 --no-install-recommends && \
  rm -Rf /var/lib/apt/lists/*  && \
  rm -Rf /usr/share/doc && rm -Rf /usr/share/man  && \
  apt-get clean

COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline

RUN mkdir /opt/database
RUN groupadd --gid 1000 canary
RUN useradd canary --uid 1000 -g canary -m -d /var/lib/canary
RUN chown -R 1000:1000 /opt/database
USER canary:canary

ENTRYPOINT ["/app/canary-checker"]
