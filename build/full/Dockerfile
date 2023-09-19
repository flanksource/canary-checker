FROM golang:1.20@sha256:690e4135bf2a4571a572bfd5ddfa806b1cb9c3dea0446ebadaf32bc2ea09d4f9 AS builder
WORKDIR /app

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=full

COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download

COPY ./ ./
RUN make build

FROM eclipse-temurin:11.0.18_10-jdk-focal@sha256:509043cc38d37a5bd44720b471c38bef40fb34de67c03baaa67a5a9d8cda52a0
WORKDIR /app
RUN apt-get update && \
  apt-get install -y curl unzip ca-certificates jq wget gnupg2 bzip2 --no-install-recommends && \
  rm -Rf /var/lib/apt/lists/*  && \
  rm -Rf /usr/share/doc && rm -Rf /usr/share/man  && \
  apt-get clean

RUN wget -q -O - https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add - && \
  echo "deb http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google.list && \
  apt-get update && apt-get install -y \
  google-chrome-stable \
  fontconfig \
  fonts-ipafont-gothic \
  fonts-wqy-zenhei \
  fonts-thai-tlwg \
  fonts-kacst \
  fonts-symbola \
  fonts-noto \
  fonts-freefont-ttf \
  --no-install-recommends

RUN apt-get update && apt-get upgrade -y && \
  rm -Rf /var/lib/apt/lists/* && \
  apt-get clean

ENV RESTIC_VERSION=0.15.2
RUN curl -L https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_amd64.bz2 -o restic.bz2 && \
  bunzip2  /app/restic.bz2 && \
  chmod +x /app/restic && \
  mv /app/restic /usr/local/bin/ && \
  rm -rf /app/restic.bz2

ENV JMETER_VERSION=5.5
RUN curl -L https://dlcdn.apache.org//jmeter/binaries/apache-jmeter-${JMETER_VERSION}.zip -o apache-jmeter-${JMETER_VERSION}.zip && \
  unzip apache-jmeter-${JMETER_VERSION}.zip -d /opt && \
  rm apache-jmeter-${JMETER_VERSION}.zip

ENV PATH /opt/apache-jmeter-${JMETER_VERSION}/bin/:$PATH

RUN curl -L https://github.com/flanksource/askgit/releases/download/v0.4.8-flanksource/askgit-linux-amd64.tar.gz -o askgit.tar.gz && \
  tar xf askgit.tar.gz && \
  mv askgit /usr/local/bin/askgit && \
  rm askgit.tar.gz && \
  wget http://mirrors.kernel.org/ubuntu/pool/main/o/openssl/openssl_1.1.1f-1ubuntu2.19_amd64.deb && \
  dpkg -i openssl_1.1.1f-1ubuntu2.19_amd64.deb && \
  rm openssl_1.1.1f-1ubuntu2.19_amd64.deb

# The best developer experience for load testing
ENV K6_VERSION=v0.44.0
RUN curl -L https://github.com/grafana/k6/releases/download/${K6_VERSION}/k6-${K6_VERSION}-linux-amd64.tar.gz -o k6.tar.gz && \
  tar xvf k6.tar.gz && \
  mv k6-${K6_VERSION}-linux-amd64/k6 /usr/local/bin/k6 && \
  rm k6.tar.gz

# Benthos is a high performance and resilient stream processor
RUN curl -Lsf https://sh.benthos.dev | bash -s -- 4.15.0

# Commandline tool for running SQL queries against JSON, CSV, Excel, Parquet, and more
RUN curl -L https://github.com/multiprocessio/dsq/releases/download/v0.23.0/dsq-linux-x64-v0.23.0.zip -o dsq.zip && \
  unzip dsq.zip && \
  mv dsq /usr/local/bin/dsq && \
  rm dsq.zip

# Install alexellis/arkade as root
RUN curl -sLS https://get.arkade.dev | sh

# Install Azure CLI (need to install as root)
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash

RUN mkdir /opt/database && groupadd --gid 1000 canary && \
  useradd canary --uid 1000 -g canary -m -d /var/lib/canary && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app

USER canary:canary

# Install CLIs with arkade as canary user
RUN arkade get kubectl stern jq yq
ENV PATH="${PATH}:/var/lib/canary/.arkade/bin/"
ENV PATH="${PATH}:/var/lib/canary/bin/"

# Install AWS CLI
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
  unzip awscliv2.zip && ./aws/install -i ${HOME}/aws -b ${HOME}/bin/ && \
  rm awscliv2.zip

# Install GCP CLI
RUN curl -sL -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-441.0.0-linux-x86_64.tar.gz && \
  tar -xf google-cloud-cli-441.0.0-linux-x86_64.tar.gz && \
  ln -sf /app/google-cloud-sdk/bin/gcloud ${HOME}/bin/gcloud && \
  rm google-cloud-cli-441.0.0-linux-x86_64.tar.gz

COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline
ENTRYPOINT ["/app/canary-checker"]
