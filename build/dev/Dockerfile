FROM ubuntu:26.10@sha256:262df65c0ec6bf8d9dcc87454f4330a05fae4b97825f45854af3b136666584c2
WORKDIR /app

RUN apt-get update && \
  apt-get install -y ca-certificates && \
  rm -Rf /var/lib/apt/lists/*  && \
  rm -Rf /usr/share/doc && rm -Rf /usr/share/man  && \
  apt-get clean
COPY  ./.bin/canary-checker  /app/canary-checker
ENTRYPOINT ["/app/canary-checker"]
