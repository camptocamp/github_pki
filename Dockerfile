FROM debian:jessie
ENV GOPATH=/go
RUN apt-get update && apt-get install -y golang-go git && \
  go get github.com/raphink/github_pki && \
  apt-get autoremove -y golang-go git && \
  apt-get clean
ENTRYPOINT ["/go/bin/github_pki"]
