FROM golang:1.20-bookworm@sha256:077ff85b374b23916b4b41835e242e5a3ddad9fc537ea7e980f230431747d245 AS builder
WORKDIR /app

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=minimal

COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download

COPY ./ ./
RUN make build

FROM ubuntu:jammy-20231004@sha256:2b7412e6465c3c7fc5bb21d3e6f1917c167358449fecac8176c6e496e5c1f05f
WORKDIR /app
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
  apt-get install -y curl unzip ca-certificates jq  tzdata wget gnupg2 bzip2 apt-transport-https lsb-release --no-install-recommends  && \
  apt-get clean

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
