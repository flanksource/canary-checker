FROM golang:1.13.6 as builder
WORKDIR /app
COPY ./ ./
ARG NAME
ARG VERSION
RUN GOOS=linux GOARCH=amd64 go build -o canary-checker -ldflags "-X \"main.version=$VERSION\""  main.go

FROM golang:1.13.6
LABEL maintainer="Vikas Saini"
COPY --from=builder /app/canary-checker /app/
COPY --from=builder /app/fixtures /app/
WORKDIR /app
ENTRYPOINT ["./canary-checker"]