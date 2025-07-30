#################################################
## Build the image with:
# docker build -t ubuntu18-build-env -f ./scripts/Ubuntu18.04.Dockerfile .
#
## Run the container with:
# docker run --rm -v ./tmp:/app/tmp -v ./packages:/app/packages ubuntu18-build-env
#
#################################################

FROM ubuntu:18.04

RUN apt-get update && \
    apt-get install -y build-essential && \
    apt-get install -y wget curl tar gzip && \
    ARCH=$([ "$(uname -m)" = "aarch64" ] && echo "arm64" || echo "amd64") && \
    echo "Building for architecture: $ARCH" && \
    echo "Install Go" && \
    wget -L https://go.dev/dl/go1.24.5.linux-$ARCH.tar.gz -O /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm -f /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app
COPY . /app

RUN go mod download && \
    go run mage.go install-neo-web

CMD ["go", "run", "mage.go", "machbase-neo", "package"]
