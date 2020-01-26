FROM golang:1.13.6 as builder
WORKDIR /app
COPY ./ ./
ARG NAME
RUN GOOS=linux GOARCH=amd64 go build -o $NAME

FROM golang:1.13.6
LABEL maintainer="Vikas Saini"
COPY --from=builder /app/$NAME /app/
COPY --from=builder /app/fixtures /app/
WORKDIR /app
ENTRYPOINT ["./$NAME"]