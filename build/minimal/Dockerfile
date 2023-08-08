FROM golang:1.20@sha256:690e4135bf2a4571a572bfd5ddfa806b1cb9c3dea0446ebadaf32bc2ea09d4f9 AS builder
WORKDIR /app

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=minimal
COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download
COPY ./ ./
RUN go version
RUN make build

FROM ubuntu 
WORKDIR /app
RUN apt-get update && \
  apt-get install -y curl unzip ca-certificates jq wget gnupg2 bzip2 --no-install-recommends && \
  rm -Rf /var/lib/apt/lists/*  && \
  rm -Rf /usr/share/doc && rm -Rf /usr/share/man  && \
  apt-get clean

COPY --from=builder /app/.bin/canary-checker /app

# Install alexellis/arkade as root
RUN curl -sLS https://get.arkade.dev | sh

# Install Azure CLI (need to install as root)
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash

RUN mkdir /opt/database
RUN groupadd --gid 1000 canary
RUN useradd canary --uid 1000 -g canary -m -d /var/lib/canary
RUN chown -R 1000:1000 /opt/database
RUN chown -R 1000:1000 /app
USER canary:canary

# Install CLIs with arkade as canary user
RUN arkade get kubectl
RUN arkade get stern
RUN arkade get jq
RUN arkade get yq
ENV PATH="${PATH}:/var/lib/canary/.arkade/bin/"
ENV PATH="${PATH}:/var/lib/canary/bin/"

# Install AWS CLI
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
RUN	unzip awscliv2.zip && ./aws/install -i ${HOME}/aws -b ${HOME}/bin/

# Install GCP CLI
RUN curl -sL -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-441.0.0-linux-x86_64.tar.gz
RUN tar -xf google-cloud-cli-441.0.0-linux-x86_64.tar.gz
RUN ln -sf /app/google-cloud-sdk/bin/gcloud ${HOME}/bin/gcloud

RUN /app/canary-checker go-offline

ENTRYPOINT ["/app/canary-checker"]
