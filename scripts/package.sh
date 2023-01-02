#!/bin/bash

set -e
PRJROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd $PRJROOT

PKGNAME="$1"
GOOS="$2"
GOARCH="$3"
VERSION="v0.0.0"

function semverIncrease() {
    TAGGED=$(git describe --tags --contains HEAD 2> /dev/null) || true
   if [ -z $TAGGED ]; then
        local RE='v[^0-9]*\([0-9]*\)[.]\([0-9]*\)[.]\([0-9]*\)'
        MAJOR=`echo $1 | sed -e "s#$RE#\1#"`
        MINOR=`echo $1 | sed -e "s#$RE#\2#"`
        PATCH=`echo $1 | sed -e "s#$RE#\3#"`
        VERSION="v$MAJOR.$MINOR.`expr $PATCH + 1`"
   fi
}

if [ -d ".git" ]; then
    tags=$(git tag | wc -l) 
    if [ $tags -gt 0 ]; then
    	VERSION=$(git describe --tags --abbrev=0)
    fi
fi

semverIncrease $VERSION

echo Packaging $PKGNAME $GOOS $GOARCH $VERSION

# Remove previous build directory, if needed.
bdir=$PKGNAME-$VERSION-$GOOS-$GOARCH
rm -rf packages/$bdir && mkdir -p packages/$bdir

if [ -d arch/$PKGNAME ]; then
    cp -R arch/$PKGNAME/* packages/$bdir/ && \
    find "packages/$bdir" -name ".gitkeep" -exec /bin/rm -f {} \;
fi
case $PKGNAME in
    machgo)
        declare -a BINS=( "machgo" )
        ;;
    *)
        declare -a BINS=( $PKGNAME )
        ;;
esac

for BIN in $BINS; do
    # Make the binaries.
    GOOS=$GOOS GOARCH=$GOARCH make $BIN

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
