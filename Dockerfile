FROM node:16 as node
WORKDIR /app
COPY "./ui/package.json" .
COPY "./ui/package-lock.json" .
RUN npm i
ADD ui/ .
RUN  ls && npm run build

FROM golang:1.17 as builder
WORKDIR /app
ARG NAME
ARG VERSION
COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download
COPY --from=node /app/build /app/ui/build
COPY ./ ./
RUN go version
RUN make build

FROM ubuntu:bionic
WORKDIR /app
# Install restic from releases
RUN apt-get update && \
  apt-get install -y curl unzip && \
  curl -L https://github.com/restic/restic/releases/download/v0.12.0/restic_0.12.0_linux_amd64.bz2 -o restic.bz2 && \
  bunzip2  /app/restic.bz2 && \
  chmod +x /app/restic && \
  mv /app/restic /usr/local/bin/ && \
  rm -rf /app/restic.bz2

#Install jmeter
RUN curl -L https://dlcdn.apache.org//jmeter/binaries/apache-jmeter-5.4.3.zip -o apache-jmeter-5.4.3.zip && \
  unzip apache-jmeter-5.4.3.zip -d /opt && \
  rm apache-jmeter-5.4.3.zip && \
  apt-get install -y openjdk-11-jre-headless

ENV PATH /opt/apache-jmeter-5.4.3/bin/:$PATH

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
RUN groupadd --gid 1000 canary
RUN useradd canary --uid 1000 -g canary -m -d /var/lib/canary
USER canary:canary
COPY --from=builder /app/.bin/canary-checker /app
ENTRYPOINT ["/app/canary-checker"]
