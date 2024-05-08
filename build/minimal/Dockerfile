FROM golang:1.22-bookworm@sha256:6d71b7c3f884e7b9552bffa852d938315ecca843dcc75a86ee7000567da0923d AS builder
WORKDIR /app

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=minimal

COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download

COPY ./ ./
RUN make build

FROM flanksource/base-image:v0.0.9
WORKDIR /app
RUN mkdir /opt/database && groupadd --gid 1000 canary && \
  useradd canary --uid 1000 -g canary -m -d /var/lib/canary && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

USER canary:canary

COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline
ENTRYPOINT ["/app/canary-checker"]
