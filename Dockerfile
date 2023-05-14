FROM golang:1.20 AS builder
WORKDIR /app

ARG NAME
ARG VERSION
COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download
COPY ./ ./
RUN go version
RUN make build

FROM eclipse-temurin:11.0.18_10-jdk-focal
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


RUN curl -L https://github.com/restic/restic/releases/download/v0.12.0/restic_0.12.0_linux_amd64.bz2 -o restic.bz2 && \
  bunzip2  /app/restic.bz2 && \
  chmod +x /app/restic && \
  mv /app/restic /usr/local/bin/ && \
  rm -rf /app/restic.bz2

#Install jmeter
RUN curl -L https://dlcdn.apache.org//jmeter/binaries/apache-jmeter-5.4.3.zip -o apache-jmeter-5.4.3.zip && \
  unzip apache-jmeter-5.4.3.zip -d /opt && \
  rm apache-jmeter-5.4.3.zip

ENV PATH /opt/apache-jmeter-5.4.3/bin/:$PATH


RUN curl -L https://github.com/flanksource/askgit/releases/download/v0.4.8-flanksource/askgit-linux-amd64.tar.gz -o askgit.tar.gz && \
    tar xf askgit.tar.gz && \
    mv askgit /usr/local/bin/askgit && \
    rm askgit.tar.gz && \
    wget http://nz2.archive.ubuntu.com/ubuntu/pool/main/o/openssl/libssl1.1_1.1.1f-1ubuntu2.18_amd64.deb && \
    dpkg -i libssl1.1_1.1.1f-1ubuntu2.18_amd64.deb && \
    rm libssl1.1_1.1.1f-1ubuntu2.18_amd64.deb

ENV K6_VERSION=v0.44.0
RUN curl -L https://github.com/grafana/k6/releases/download/${K6_VERSION}/k6-${K6_VERSION}-linux-amd64.tar.gz -o k6.tar.gz && \
    tar xvf k6.tar.gz && \
    mv k6-${K6_VERSION}-linux-amd64/k6 /usr/local/bin/k6 && \
    rm k6.tar.gz

RUN curl -Lsf https://sh.benthos.dev | bash -s -- 3.56.0

RUN curl -L https://github.com/multiprocessio/dsq/releases/download/v0.23.0/dsq-linux-x64-v0.23.0.zip -o dsq.zip && \
    unzip dsq.zip && \
    mv dsq /usr/local/bin/dsq && \
    rm dsq.zip

RUN curl -L https://github.com/stern/stern/releases/download/v1.25.0/stern_1.25.0_linux_amd64.tar.gz -o stern.tar.gz && \
    tar xvf stern.tar.gz && \
    mv stern /usr/local/bin/stern && \
    rm stern.tar.gz

# install CA certificates
COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline

RUN mkdir /opt/database
RUN groupadd --gid 1000 canary
RUN useradd canary --uid 1000 -g canary -m -d /var/lib/canary
RUN chown -R 1000:1000 /opt/database
USER canary:canary

ENTRYPOINT ["/app/canary-checker"]
