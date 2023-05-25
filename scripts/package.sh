#!/bin/bash

set -e
PRJROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd $PRJROOT
PRJABSPATH=`pwd`

PKGNAME="$1"
GOOS="$2"
GOARCH="$3"
VERSION="$4"
EDITION="$5"

echo Packaging $PKGNAME $GOOS $GOARCH $VERSION $EDITION

# Remove previous build directory, if needed.
bdir=$PKGNAME-$EDITION-$VERSION-$GOOS-$GOARCH
if [ "$GOARCH" == "arm" ]; then
    bdir="$PKGNAME-$EDITION-$VERSION-$GOOS-arm32"
fi

# neoshell does not have edition
if [ "$PKGNAME" == "neoshell" ]; then
    bdir="$PKGNAME-$VERSION-$GOOS-$GOARCH"
fi

echo "    prepare dir $bdir"
rm -rf packages/$bdir && mkdir -p packages/$bdir

if [ -d arch/$PKGNAME ]; then
    cp -R arch/$PKGNAME/* packages/$bdir/ && \
    find "packages/$bdir" -name ".gitkeep" -exec /bin/rm -f {} \;
fi
case $PKGNAME in
    "machbase-neo")
        declare -a BINS=( "machbase-neo" )
        ;;
    "neow")
        declare -a BINS=( "machbase-neo" "neow" )
        ;;
    *)
        declare -a BINS=( $PKGNAME )
        ;;
esac

for BIN in "${BINS[@]}"; do
    echo "    make $BIN $VERSION $EDITION"
    # Make the binaries.
    if [ "$GOARCH" == "arm" ]; then
        GOOS=$GOOS GOARCH=$GOARCH EDITION=$EDITION ./scripts/buildx.sh $PKGNAME $EDITION $GOOS $GOARCH
    else
        GOOS=$GOOS GOARCH=$GOARCH EDITION=$EDITION make $BIN
    fi
    # Copy the executable binaries.
    if [ "$GOOS" == "windows" ]; then
        mv tmp/$BIN packages/$bdir/$BIN.exe
    else
        mv tmp/$BIN packages/$bdir/
    fi
done

echo "    archiving $bdir.zip"

# Copy documention and license.
for D in $DOCS; do
    cp $D packages/$bdir
done

# Compress the package.
if [ "$GOOS" == "darwin" ] && [ "$PKGNAME" == "neow" ]; then
    if [ -d neow.app ]; then
        rm -rf neow.app
    fi
    if [ -d packages/neow.app ]; then
        rm -rf packages/neow.app
    fi
    fyne package --os darwin --src main/neow --icon $PRJABSPATH/main/neow/res/appicon.png --id com.machbase.neow && \
    mv neow.app packages/ && \
    mv packages/$bdir/machbase-neo packages/neow.app/Contents/MacOS/
    cd packages
    zip -r -q neow-$EDITION-$VERSION-$GOOS-$GOARCH.zip neow.app && rm -rf neow.app
else
    cd packages
    zip -r -q $bdir.zip $bdir
fi

# Remove build directory.
rm -rf $bdir

echo "Packaging done."