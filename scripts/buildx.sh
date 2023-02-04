#!/bin/bash

set -e
PRJROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd $PRJROOT

# Check the Go installation
if [ "$(which go)" == "" ]; then
	echo "error: Go is not installed. Please download and follow installation"\
		 "instructions at https://golang.org/dl to continue."
	exit 1
fi

MODNAME=`cat $PRJROOT/go.mod | grep "^module " | awk '{print $2}'`

# ex)
# ./scripts/buildx.sh machbase-neo edge linux arm64
# ./scripts/buildx.sh machbase-neo edge darwin arm64

# 1st Target
if [ "$1" == "" ]; then
    echo "error: missing argument (target name)"
    exit 1
fi

# 2nd Edition
if [ "$2" == "" ]; then
    EDITION="edge"
else
    EDITION="$2"
fi

# 3rd OS
X_OS="$3"

# 4th Arch
X_ARCH="$4"

VERSION=`$PRJROOT/scripts/buildversion.sh`

# Hardcode some values to the core package.
if [ -f ".git" ]; then
	GITSHA=$(git rev-parse --short `git branch --show-current`)
fi

echo "Build $MODNAME $EDITION $VERSION $GITSHA"

LDFLAGS="$LDFLAGS -X $MODNAME/mods.versionGitSHA=${GITSHA}"
GOVERSTR=$(go version | sed -r 's/go version go(.*)\ .*/\1/')
LDFLAGS="$LDFLAGS -X $MODNAME/mods.versionString=${VERSION}"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.goVersionString=${GOVERSTR}"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.buildTimestamp=$(date "+%Y-%m-%dT%H:%M:%S")"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.editionString=${EDITION}"

#
# refere to https://ziglang.org/documentation/0.10.1/#Targets
#
case $X_OS in
    linux)
        if [ $X_ARCH == "arm64" ]; then
            X_CC="zig cc -target aarch64-linux-gnu"
            X_CX="zig c++ -target aarch64-linux-gnu"
        elif [ $X_ARCH == "amd64" ]; then
            X_CC="zig cc -target x86_64-linux-gnu"
            X_CX="zig c++ -target x86_64-linux-gnu"
        elif [ $X_ARCH == "386" ]; then
            X_CC="zig cc -target i386-linux-gnu"
            X_CX="zig c++ -target i386-linux-gnu"
        else
            echo "supported linux/$X_ARCH"
            exit 1
        fi
    ;;
    darwin)
        SYSROOT=`xcrun --sdk macosx --show-sdk-path`
        SYSFLAGS="-v --sysroot=$SYSROOT -I/usr/include, -F/System/Library/Frameworks -L/usr/lib"
        if [ $X_ARCH == "arm64" ]; then
            X_CC="zig cc -target aarch64-macos.13-none $SYSFLAGS"
            X_CX="zig c++ -target aarch64-macos.13-none $SYSFLAGS"
        elif [ $X_ARCH == "amd64" ]; then
            X_CC="zig cc -target x86_64-macos.13-none $SYSFLAGS"
            X_CX="zig c++ -target x86_64-macos.13-none $SYSFLAGS"
        else
            echo "supported darwin/$X_ARCH"
            exit 1
        fi
    ;;
    windows)
        X_CC="zig cc -target x86_64-windows-none"
        X_CX="zig c++ -target x86_64-windows-none"
    ;;
    *)
        echo "supported os $X_OS"
        exit 1
    ;;
esac

# Build and store objects into original directory.
GO111MODULE=auto \
CGO_ENABLED=1 \
GOOS=$X_OS \
GOARCH=$X_ARCH \
CC=$X_CC \
CXX=$X_CXX \
go build \
    -ldflags "$LDFLAGS" \
    -tags ${EDITION}_edition \
    -o $PRJROOT/tmp/$1 \
    ./main/$1
