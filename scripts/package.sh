#!/bin/bash

set -e
PRJROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd $PRJROOT

PKGNAME="$1"
GOOS="$2"
GOARCH="$3"
VERSION="$4"
EDITION="$5"

echo Packaging $PKGNAME $GOOS $GOARCH $VERSION $EDITION

# Remove previous build directory, if needed.
bdir=$PKGNAME-$EDITION-$VERSION-$GOOS-$GOARCH
rm -rf packages/$bdir && mkdir -p packages/$bdir

if [ -d arch/$PKGNAME ]; then
    cp -R arch/$PKGNAME/* packages/$bdir/ && \
    find "packages/$bdir" -name ".gitkeep" -exec /bin/rm -f {} \;
fi
case $PKGNAME in
    *)
        declare -a BINS=( $PKGNAME )
        ;;
esac

for BIN in $BINS; do
    # Make the binaries.
    GOOS=$GOOS GOARCH=$GOARCH EDITION=$EDITION make $BIN

    # Copy the executable binaries.
    if [ "$GOOS" == "windows" ]; then
        mv tmp/$BIN packages/$bdir/$BIN.exe
    else
        mv tmp/$BIN packages/$bdir/
    fi
done


# Copy documention and license.
for D in $DOCS; do
    cp $D packages/$bdir
done

# Copy test directory
# if [ ! -d packages/$bdir/test ]; then
#     mkdir packages/$bdir/test
# fi
# for D in $TESTD; do
#     cp -r $D packages/$bdir/test
# done

# Compress the package.
cd packages
zip -r -q $bdir.zip $bdir
# if [ "$GOOS" == "linux" ]; then
# 	tar -zcf $bdir.tar.gz $bdir
# else
# 	zip -r -q $bdir.zip $bdir
# fi

# Remove build directory.
rm -rf $bdir
