FROM golang:1.20-bookworm@sha256:077ff85b374b23916b4b41835e242e5a3ddad9fc537ea7e980f230431747d245 AS builder
WORKDIR /app

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=full

COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download

COPY ./ ./
RUN make build

FROM eclipse-temurin:11.0.20.1_1-jdk-jammy@sha256:1584fd589b45a67b6f56b0c702776ca3d5640d1001f7848f5bcd19cb10545eaa
WORKDIR /app
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
  apt-get install -y curl unzip ca-certificates jq tzdata wget gnupg2 bzip2 apt-transport-https lsb-release python3 python3-pip --no-install-recommends && \
  rm -Rf /var/lib/apt/lists/*  && \
  rm -Rf /usr/share/doc && rm -Rf /usr/share/man  && \
  apt-get clean

RUN pip3 install  pip pyyaml lxml requests robotframework \
    robotframework \
    robotframework-jsonlibrary \
    robotframework-jsonschemalibrary \
    robotframework-requests \
    robotframework-restlibrary \
    robotframework-seleniumlibrary \
    robotframework-excellib \
    robotframework-crypto \
    robotframework-databaselibrary \
    psycopg2-binary \
    PyMySQL

RUN mkdir -p /etc/apt/keyrings && \
  curl -sLS https://packages.microsoft.com/keys/microsoft.asc | \
    gpg --dearmor | tee /etc/apt/keyrings/microsoft.gpg > /dev/null && \
  chmod go+r /etc/apt/keyrings/microsoft.gpg &&  \
  echo "deb [arch=`dpkg --print-architecture` signed-by=/etc/apt/keyrings/microsoft.gpg] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/azure-cli.list && \
  cat /etc/apt/sources.list.d/azure-cli.list && \
  apt-get update && \
  apt-get install -y azure-cli && \
  apt-get clean

RUN apt-get update && apt-get upgrade -y && \
  rm -Rf /var/lib/apt/lists/* && \
  apt-get clean

ENV RESTIC_VERSION=0.15.2
RUN curl -L https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_amd64.bz2 -o restic.bz2 && \
  bunzip2  /app/restic.bz2 && \
  chmod +x /app/restic && \
  mv /app/restic /usr/local/bin/ && \
  rm -rf /app/restic.bz2

ENV JMETER_VERSION=5.6.2
RUN curl -L https://dlcdn.apache.org//jmeter/binaries/apache-jmeter-${JMETER_VERSION}.zip -o apache-jmeter-${JMETER_VERSION}.zip && \
  unzip -q apache-jmeter-${JMETER_VERSION}.zip -d /opt && \
  rm apache-jmeter-${JMETER_VERSION}.zip

ENV PATH /opt/apache-jmeter-${JMETER_VERSION}/bin/:$PATH

RUN curl -L https://github.com/flanksource/askgit/releases/download/v0.61.0-flanksource.1/mergestat-linux-amd64.tar.gz  -o mergestat.tar.gz && \
  tar zxf mergestat.tar.gz -C /usr/local/bin/ && \
  rm mergestat.tar.gz

# The best developer experience for load testing
ENV K6_VERSION=v0.47.0
RUN curl -L https://github.com/grafana/k6/releases/download/${K6_VERSION}/k6-${K6_VERSION}-linux-amd64.tar.gz -o k6.tar.gz && \
  tar xvf k6.tar.gz && \
  mv k6-${K6_VERSION}-linux-amd64/k6 /usr/local/bin/k6 && \
  rm k6.tar.gz

# Benthos is a high performance and resilient stream processor
RUN curl -Lsf https://sh.benthos.dev | bash -s -- 4.22.0

# Commandline tool for running SQL queries against JSON, CSV, Excel, Parquet, and more
RUN curl -L https://github.com/multiprocessio/dsq/releases/download/v0.23.0/dsq-linux-x64-v0.23.0.zip -o dsq.zip && \
  unzip -q dsq.zip && \
  mv dsq /usr/local/bin/dsq && \
  rm dsq.zip

# Install alexellis/arkade as root
RUN curl -sLS https://get.arkade.dev | sh


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
  unzip -q awscliv2.zip && ./aws/install -i ${HOME}/aws -b ${HOME}/bin/ && \
  rm awscliv2.zip

# Install GCP CLI
ENV GCLOUD_VERSION=441.0.0
RUN curl -sL -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-${GCLOUD_VERSION}-linux-x86_64.tar.gz && \
  tar -xf google-cloud-cli-${GCLOUD_VERSION}-linux-x86_64.tar.gz && \
  ln -sf /app/google-cloud-sdk/bin/gcloud ${HOME}/bin/gcloud && \
  rm google-cloud-cli-${GCLOUD_VERSION}-linux-x86_64.tar.gz

COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline
ENTRYPOINT ["/app/canary-checker"]
