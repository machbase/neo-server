#!/bin/bash

set -e
PRJROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd $PRJROOT

MODNAME=`cat $PRJROOT/go.mod | grep "^module " | awk '{print $2}'`
#MODNAME="github.com/machbase/neo-engine"

if [ "$1" == "" ]; then
    echo "error: missing argument (target name)"
    exit 1
fi

VERSION="$2"

# Check the Go installation
if [ "$(which go)" == "" ]; then
	echo "error: Go is not installed. Please download and follow installation"\
		 "instructions at https://golang.org/dl to continue."
	exit 1
fi

echo "Build version $MODNAME $VERSION"

# Hardcode some values to the core package.
if [ -d ".git" ]; then
	GITSHA=$(git rev-parse --short HEAD)
	LDFLAGS="$LDFLAGS -X $MODNAME/mods.versionString=${VERSION}"
	LDFLAGS="$LDFLAGS -X $MODNAME/mods.versionGitSHA=${GITSHA}"
fi
GOVERSTR=$(go version | sed -r 's/go version go(.*)\ .*/\1/')
LDFLAGS="$LDFLAGS -X $MODNAME/mods.goVersionString=${GOVERSTR}"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.buildTimestamp=$(date "+%Y-%m-%dT%H:%M:%S")"

# Set final Go environment options
#LDFLAGS="$LDFLAGS -extldflags '-static'"

export CGO_ENABLED=1

if [ "$NOMODULES" != "1" ]; then
	export GO111MODULE=on
    if [ ! -d ./vendor ]; then
	    go mod vendor
    fi
fi

# Build and store objects into original directory.
go build -ldflags "$LDFLAGS" -o $PRJROOT/tmp/$1 ./main/$1
