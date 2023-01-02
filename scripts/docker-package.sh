#!/bin/bash

# PKGNAME="machgo"
PKGNAME=$1
# OS="linux"
OS=$2
# ARCH="arm64"
ARCH=$3

IMAGE="${PKGNAME}_buildenv_${OS}_${ARCH}:latest"

docker image inspect $IMAGE --format "Check $IMAGE exists." 2> /dev/null
exists=$?

set -e

if [ $exists -ne 0 ]; then
    echo "Creating docker image '${IMAGE}' for build environment ..."
    docker buildx build -f scripts/buildenv-dockerfile --rm -t $IMAGE --platform=$OS/$ARCH .
fi

if [ ! -f ~/.netrc ]; then
    echo "~/.netrc not found, it is required to check-out dependency modules"
    exit 1
fi

echo "Build package via $IMAGE"
docker run \
    --rm \
    --platform $OS/$ARCH \
    -v "$PWD":/machgo \
    -w /machgo \
    -v "$HOME/.netrc":/root/.netrc \
    -v "$HOME/go:/root/go" \
    -e GOPRIVATE="github.com/machbase/*" \
    $IMAGE \
    /bin/bash -c "git config --global --add safe.directory '*' && ./scripts/package.sh $PKGNAME $OS $ARCH"
