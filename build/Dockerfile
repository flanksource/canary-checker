FROM golang:1.13.6 as builder
WORKDIR /app
COPY ./ ./
RUN GOOS=linux GOARCH=amd64 go build -o main

FROM golang:1.13.6
LABEL maintainer="Vikas Saini"
COPY --from=builder /app/main /app/
COPY --from=builder /app/fixtures /app/
WORKDIR /app
ENTRYPOINT ["./main"]