#################################################
## Prerequisites - qemu
# sudo apt-get update
# sudo apt-get install qemu-system-x86 binfmt-support qemu-user-static
#
## Build the machbase-neo image with:
# docker build --platform linux/arm64 -t machbase-neo-arm64 -f ./scripts/machbase-neo.Dockerfile .
# docker build --platform linux/amd64 -t machbase-neo-amd64 -f ./scripts/machbase-neo.Dockerfile .
#
## Build the machbase-neo image for multiple platforms:
# docker buildx build --platform linux/arm64,linux/amd64 -t machbase/machbase-neo -f ./scripts/machbase-neo.Dockerfile .
#################################################

#################################################
## Build Stage
# This stage is used to build the machbase-neo binary.
#################################################

FROM ubuntu:22.04 AS build-stage

RUN apt-get update && \
    apt-get install -y build-essential && \
    apt-get install -y wget curl tar gzip && \
    MACHINE=$(uname -m) && \
    case $MACHINE in \
    x86_64) ARCH="amd64" ;; \
    aarch64) ARCH="arm64" ;; \
    armv7l) ARCH="armv6l" ;; \
    *) echo "Unsupported architecture: $MACHINE" && exit 1 ;; \
    esac && \
    echo "Building for architecture: $ARCH" && \
    echo "Install Go" && \
    curl -fsSL --retry 3 --retry-delay 5 --connect-timeout 30 \
    https://go.dev/dl/go1.24.5.linux-$ARCH.tar.gz -o /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm -f /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app
COPY . /app

RUN go mod download && \
    go run mage.go install-neo-web && \
    go run mage.go machbase-neo && \
    mkdir -p /opt && \
    cp ./tmp/machbase-neo /opt/machbase-neo && \
    chmod +x /opt/machbase-neo

#################################################
## Runtime Stage
# This stage is used to create the final image 
# that will run the machbase-neo server.
#################################################
FROM ubuntu:22.04 AS runtime-stage

LABEL MAINTAINER="machbase.com <support@machbase.com>"

RUN apt-get update && \
    apt-get install -y ca-certificates && \
    update-ca-certificates && \
    mkdir -p /file /data /backups

COPY --from=build-stage /opt/machbase-neo /opt/machbase-neo

EXPOSE 5652-5656

VOLUME ["/data", "/file", "/backups"]

ENTRYPOINT ["/opt/machbase-neo", \
    "serve", \
    "--host", "0.0.0.0", \
    "--data", "/data", \
    "--file", "/file", \
    "--backup-dir", "/backups"]

