FROM golang:1.21@sha256:02d7116222536a5cf0fcf631f90b507758b669648e0f20186d2dc94a9b419a9b AS builder
WORKDIR /app

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=minimal

COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download

COPY ./ ./
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

RUN /app/canary-checker go-offline

ENTRYPOINT ["/app/canary-checker"]
