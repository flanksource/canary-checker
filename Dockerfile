FROM node:12 as node
WORKDIR /app
COPY ./ ./
RUN make vue-dist

FROM golang:1.13.6
WORKDIR /app
COPY ./ ./
ARG NAME
ARG VERSION
RUN go get -u github.com/mjibson/esc
COPY --from=node /app/statuspage/dist /app/statuspage/
WORKDIR /app/statuspage/dist
RUN	 esc -o ../static.go -pkg statuspage .
WORKDIR /app
RUN GOOS=linux GOARCH=amd64 go build -o canary-checker -ldflags "-X \"main.version=$VERSION\""  main.go

FROM ubuntu:bionic
WORKDIR /app
ADD /.bin/canary-checker /app
ENTRYPOINT ["/app/canary-checker"]
