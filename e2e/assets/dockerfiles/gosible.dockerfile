FROM ubuntu:22.04

WORKDIR /setup_ground

COPY .go-version .
RUN apt-get update && \
    apt-get install -y openssh-client curl make tar zip patch && \
    curl -OL https://go.dev/dl/go$(cat .go-version).linux-amd64.tar.gz && \
    rm -rf /usr/local/go && tar -C /usr/local -xzf go$(cat .go-version).linux-amd64.tar.gz && \
    cp /usr/local/go/bin/go /usr/local/bin && \
    cp /usr/local/go/bin/gofmt /usr/local/bin

COPY e2e/assets/ssh /root/.ssh
RUN chmod -R 600 /root/.ssh

WORKDIR /build_ground

COPY . .
RUN go mod download -x && \
    ./tools/download-ansible-py-modules.sh && \
    make build && \
    cp bin/gosible /usr/local/bin && \
    mkdir /usr/local/bin/remote && \
    cp bin/remote/gosible_client /usr/local/bin/remote && \
    cp bin/remote/py_runtime.zip /usr/local/bin/remote

WORKDIR /test_ground

CMD ["sleep", "infinity"]
