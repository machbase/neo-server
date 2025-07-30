#################################################
## Build the image with:
# docker build -t ubuntu24-build-env -f ./scripts/Ubuntu24.04.Dockerfile .
#
## Run the container with:
# docker run --rm -v ./tmp:/app/tmp -v ./packages:/app/packages ubuntu24-build-env
#
#################################################

FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y build-essential && \
    apt-get install -y wget curl tar gzip && \
    ARCH=$([ "$(uname -m)" = "aarch64" ] && echo "arm64" || echo "amd64") && \
    echo "Building for architecture: $ARCH" && \
    echo "Install Go" && \
    wget -L https://go.dev/dl/go1.24.5.linux-$ARCH.tar.gz -O /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm -f /tmp/go.tar.gz && \
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc

WORKDIR /app
COPY . /app

RUN /usr/local/go/bin/go mod download && \
    /usr/local/go/bin/go run mage.go install-neo-web

CMD ["/usr/local/go/bin/go", "run", "mage.go", "machbase-neo", "package"]

