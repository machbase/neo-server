#################################################
## Prerequisites - qemu
# sudo apt-get update
# sudo apt-get install qemu-system-x86 binfmt-support qemu-user-static
#
## Build the image with:
# docker build --platform linux/arm/v7 -t ubuntu_arm32v7-build-env -f ./scripts/Ubuntu_arm32v7.Dockerfile .
#
## Run the container with:
# docker run --rm -v ./tmp:/app/tmp -v ./packages:/app/packages --platform linux/arm/v7 ubuntu_arm32v7-build-env
#
#################################################

## Base Image
#    https://hub.docker.com/r/arm32v7/ubuntu/
# Note: This image is based on Ubuntu 22.04 (Jammy Jellyfish)
# The image is built for ARMv7 architecture (arm32v7)
# The image is suitable for running on ARMv7 devices such as Raspberry Pi 2/3/4, BeagleBone, etc.
FROM arm32v7/ubuntu:jammy

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
    go run mage.go install-neo-web

CMD ["go", "run", "mage.go", "machbase-neo", "package"]
