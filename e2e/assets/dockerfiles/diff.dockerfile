FROM ubuntu:22.04

RUN apt update && \
    apt install -y curl && \
    curl -LO https://storage.googleapis.com/container-diff/latest/container-diff-linux-amd64 && \
    install container-diff-linux-amd64 /usr/local/bin/container-diff

CMD ["sleep", "infinity"]
