#!/bin/bash

set -e
PRJROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd $PRJROOT

MODNAME=`cat $PRJROOT/go.mod | grep "^module " | awk '{print $2}'`
echo "    set mod $MODNAME"

if [ "$1" == "" ]; then
    echo "error: missing argument (target name)"
    exit 1
fi
echo "    set target $1"

VERSION="$2"
echo "    set version $VERSION"

# Check the Go installation
if [ "$(which go)" == "" ]; then
	echo "error: Go is not installed. Please download and follow installation"\
		 "instructions at https://golang.org/dl to continue."
	exit 1
fi

EDITION="standard"
echo "    set edition $EDITION"

BRANCH=$(git branch --show-current)
if [ -z $BRANCH ]; then
    BRANCH="HEAD"
fi
echo "    set branch $BRANCH"

if [ -d ".git" ]; then
	GITSHA=$(git rev-parse --short $BRANCH)
else
    if [ -f ".git" ]; then
    	GITSHA=$(git rev-parse --short $BRANCH)
    else
        GITSHA="-"
    fi
fi
echo "    set gitsha $GITSHA"

echo "Build $MODNAME $1 $EDITION $VERSION $GITSHA"

GOVERSTR=$(go version | sed -r 's/go version go(.*)\ .*/\1/')
LDFLAGS="$LDFLAGS -X $MODNAME/mods.goVersionString=${GOVERSTR}"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.versionString=${VERSION}"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.versionGitSHA=${GITSHA}"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.editionString=${EDITION}"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.buildTimestamp=$(date "+%Y-%m-%dT%H:%M:%S")"

if [ "$NOMODULES" != "1" ]; then
	export GO111MODULE=on
    go mod tidy
    # if [ ! -d ./vendor ]; then
	#     go mod vendor
    # fi
fi

# Build and store objects into original directory.
if [ "$1" == "neoshell" ]; then
    CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -tags "neoshell" -o $PRJROOT/tmp/$1 ./main/$1 && \
    echo "Build done."
elif [ "$1" == "neow" ]; then
    CGO_ENABLED=1 go build -o $PRJROOT/tmp/$1 ./main/$1 && \
    echo "Build done."
else
    # Set final Go environment options
    export CGO_ENABLED=1
    go build -ldflags "$LDFLAGS" -tags "${EDITION}_edition" -o $PRJROOT/tmp/$1 ./main/$1 && \
    echo "Build done."
fi
