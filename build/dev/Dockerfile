FROM ubuntu:20.04@sha256:f2034e7195f61334e6caff6ecf2e965f92d11e888309065da85ff50c617732b8
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

RUN apt-get update && \
  apt-get install -y ca-certificates && \
  rm -Rf /var/lib/apt/lists/*  && \
  rm -Rf /usr/share/doc && rm -Rf /usr/share/man  && \
  apt-get clean
COPY  ./.bin/canary-checker  /app/canary-checker
ENTRYPOINT ["/app/canary-checker"]
