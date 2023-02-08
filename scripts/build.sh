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

if [ "$3" == "" ]; then
    EDITION="fog"
else
    EDITION="$3"
fi
echo "    set edition $EDITION"

BRANCH=$(git branch --show-current)
if [ -z $BRANCH ]; then
    BRANCH="HEAD"
fi
echo "    set edition $BRANCH"

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

LDFLAGS="$LDFLAGS -X $MODNAME/mods.versionGitSHA=${GITSHA}"
GOVERSTR=$(go version | sed -r 's/go version go(.*)\ .*/\1/')
LDFLAGS="$LDFLAGS -X $MODNAME/mods.versionString=${VERSION}"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.goVersionString=${GOVERSTR}"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.buildTimestamp=$(date "+%Y-%m-%dT%H:%M:%S")"
LDFLAGS="$LDFLAGS -X $MODNAME/mods.editionString=${EDITION}"

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
go build -ldflags "$LDFLAGS" -tags "${EDITION}_edition" -o $PRJROOT/tmp/$1 ./main/$1 && \
echo "Build done."
