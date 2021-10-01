FROM node:16 as node
WORKDIR /app
COPY "./ui/package.json" .
COPY "./ui/package-lock.json" .
RUN npm i
ADD ui/ .
RUN  ls && npm run build

FROM golang:1.16 as builder
WORKDIR /app
COPY ./ ./
ARG NAME
ARG VERSION
COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download
COPY --from=node /app/build /app/ui/build
WORKDIR /app
RUN go version
RUN GOOS=linux GOARCH=amd64 go build -o canary-checker -ldflags "-X \"main.version=$VERSION\""  main.go

FROM ubuntu:bionic
WORKDIR /app
# Install restic from releases
RUN apt-get update && \
  apt-get install -y curl && \
  curl -L https://github.com/restic/restic/releases/download/v0.12.0/restic_0.12.0_linux_amd64.bz2 -o restic.bz2 && \
  bunzip2  /app/restic.bz2 && \
  chmod +x /app/restic && \
  mv /app/restic /usr/local/bin/ && \
  rm -rf /app/restic.bz2

#Install jmeter
RUN curl -L https://mirrors.estointernet.in/apache//jmeter/binaries/apache-jmeter-5.4.1.tgz -o apache-jmeter-5.4.1.tgz && \
  tar xf apache-jmeter-5.4.1.tgz -C / && \
  rm /app/apache-jmeter-5.4.1.tgz && \
  apt-get install -y openjdk-11-jre-headless

ENV PATH /apache-jmeter-5.4.1/bin/:$PATH

# Install askgit binaries
RUN curl -L https://github.com/flanksource/askgit/releases/download/v0.4.8-flanksource/askgit-linux-amd64.tar.gz -o askgit.tar.gz && \
    tar xf askgit.tar.gz && \
    mv askgit /usr/local/bin/askgit && \
    rm askgit.tar.gz

# install CA certificates
RUN apt-get update && \
  apt-get install -y ca-certificates && \
  rm -Rf /var/lib/apt/lists/*  && \
  rm -Rf /usr/share/doc && rm -Rf /usr/share/man  && \
  apt-get clean
COPY --from=builder /app/canary-checker /app
ENTRYPOINT ["/app/canary-checker"]
