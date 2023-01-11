VERSION=$(git describe --tags --abbrev=0 2> /dev/null)
if [ -z $VERSION ]; then
    VERSION="v0.0.0"
fi

TAGGED=$(git describe --tags --contains HEAD 2> /dev/null) || true
if [ -z $TAGGED ]; then
    RE='v[^0-9]*\([0-9]*\)[.]\([0-9]*\)[.]\([0-9]*\)'
    MAJOR=`echo $VERSION | sed -e "s#$RE#\1#"`
    MINOR=`echo $VERSION | sed -e "s#$RE#\2#"`
    PATCH=`echo $VERSION | sed -e "s#$RE#\3#"`
    VERSION="v$MAJOR.$MINOR.`expr $PATCH + 1`"
fi
echo $VERSION