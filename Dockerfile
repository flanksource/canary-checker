FROM node:16 as node
WORKDIR /app
ARG GITHUB_TOKEN
COPY ./ ./
RUN echo //npm.pkg.github.com/:_authToken=$GITHUB_TOKEN >> ui/.npmrc
RUN make ui
RUN rm ui/.npmrc


FROM golang:1.16 as builder
WORKDIR /app
COPY ./ ./
ARG NAME
ARG VERSION
COPY --from=node /app/ui/build /app/ui/build
WORKDIR /app/ui/build
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

# install CA certificates
RUN apt-get update && \
  apt-get install -y ca-certificates && \
  rm -Rf /var/lib/apt/lists/*  && \
  rm -Rf /usr/share/doc && rm -Rf /usr/share/man  && \
  apt-get clean
COPY --from=builder /app/canary-checker /app
ENTRYPOINT ["/app/canary-checker"]
