FROM golang:1.22-bookworm@sha256:b03f3ba515751657c75475b20941fef47341fccb3341c3c0b64283ff15d3fb46 AS builder
WORKDIR /app

ARG NAME
ARG VERSION
ENV IMAGE_TYPE=full

COPY go.mod /app/go.mod
COPY go.sum /app/go.sum
RUN go mod download

COPY ./ ./
RUN make build

FROM flanksource/base-image:v0.0.9
WORKDIR /app

RUN apt-get update && \
  apt-get install -y python3 python3-pip zip --no-install-recommends && \
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

ENV RESTIC_VERSION=0.15.2
RUN curl -L https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_amd64.bz2 -o restic.bz2 && \
  bunzip2  /app/restic.bz2 && \
  chmod +x /app/restic && \
  mv /app/restic /usr/local/bin/ && \
  rm -rf /app/restic.bz2

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

RUN mkdir /opt/database && groupadd --gid 1000 canary && \
  useradd canary --uid 1000 -g canary -m -d /var/lib/canary && \
  chown -R 1000:1000 /opt/database && chown -R 1000:1000 /app


ENV SDKMAN_DIR="/usr/lib/sdkman"

ENV JAVA_VERSION=21.0.2-tem
ENV JMETER_VERSION=5.6
ENV JMETER_HOME=${SDKMAN_DIR}/candidates/jmeter/${JMETER_VERSION}
ENV JAVA_HOME=${SDKMAN_DIR}/candidates/java/${JAVA_VERSION}
ENV PATH="$PATH:$SDKMAN_DIR/bin:$JAVA_HOME/bin:$JMETER_HOME/bin"
SHELL [ "bash", "-c" ]
RUN  curl -s "https://get.sdkman.io?rcupdate=false" | bash && \
    . $SDKMAN_DIR/bin/sdkman-init.sh && \
    sdk install java ${JAVA_VERSION} ${JAVA_HOME} && \
    sdk install jmeter ${JMETER_VERSION} ${JMETER_HOME} && \
    sdk flush tmp

USER canary:canary

ENV PATH="${PATH}:/var/lib/canary/bin/"

COPY --from=builder /app/.bin/canary-checker /app

RUN /app/canary-checker go-offline
ENTRYPOINT ["/app/canary-checker"]
