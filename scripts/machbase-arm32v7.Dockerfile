#################################################
## Prerequisites - qemu
# sudo apt-get update
# sudo apt-get install qemu-system-x86 binfmt-support qemu-user-static
#
## Build the machbase-neo image with:
# docker build --platform linux/arm/v7 -t machbase-neo-arm32v7 -f ./scripts/machbase-arm32v7.Dockerfile .
#
## Run the machbase-neo container with:
# docker run -p 15654:5654 -p 15656:5656 --platform linux/arm/v7 machbase-neo-arm32v7
#
#################################################

#################################################
## Build Stage
# This stage is used to build the machbase-neo binary.
#################################################
FROM arm32v7/ubuntu:jammy AS build-stage

RUN apt-get update && \
    apt-get install -y build-essential && \
    apt-get install -y wget curl tar gzip && \
    echo "Install Go" && \
    wget -L https://go.dev/dl/go1.24.5.linux-armv6l.tar.gz -O /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm -f /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app
COPY . /app

RUN go mod download && \
    go run mage.go install-neo-web && \
    go run mage.go machbase-neo && \
    cp ./tmp/machbase-neo /opt/machbase-neo

#################################################
## Runtime Stage
# This stage is used to create the final image 
# that will run the machbase-neo server.
#################################################
FROM arm32v7/ubuntu:jammy AS runtime-stage

LABEL MAINTAINER="machbase.com <support@machbase.com>"

RUN apt-get update && apt-get install -y ca-certificates
RUN mkdir -p /file /data /backups

COPY --from=build-stage /opt/machbase-neo /opt/machbase-neo

EXPOSE 5652-5656

VOLUME ["/data", "/file", "/backups"]

ENTRYPOINT /opt/machbase-neo serve \
           --host 0.0.0.0 \
           --data /data \
           --file /file \
           --backup-dir /backups

