VERSION=$(git describe --tags --abbrev=0 2> /dev/null)
if [ -z $VERSION ]; then
    VERSION="v0.0.0"
fi

# 'main' branch -> use the latest tag 
# 'devel-v1.2.3' branch -> get version from branch name

BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ $BRANCH == devel-* ]]; then
    VERSION=$(echo $BRANCH | sed -n 's/devel-\(v[0-9.]*\).*/\1/p')
    MAJOR=`echo $VERSION | sed -n 's/v\([0-9]*\)*.*/\1/p'`
    MINOR=`echo $VERSION | sed -ne 's/v[0-9]*[.]\([0-9]*\).*/\1/p'`
    PATCH=`echo $VERSION | sed -ne 's/v[0-9]*[.][0-9]*[.]\([0-9]*\).*/\1/p'`
    if [ -z $PATCH ]; then
        VERSION="v$MAJOR.$MINOR-devel"
    else
        PATCH=`expr $PATCH + 0`
        VERSION="v$MAJOR.$MINOR.$PATCH-devel"
    fi
else
    TAGGED=$(git describe --tags --contains HEAD 2> /dev/null) || true
    if [ -z $TAGGED ]; then
        RE='v[^0-9]*\([0-9]*\)[.]\([0-9]*\)[.]\([0-9]*\)'
        MAJOR=`echo $VERSION | sed -e "s#$RE#\1#"`
        MINOR=`echo $VERSION | sed -e "s#$RE#\2#"`
        PATCH=`echo $VERSION | sed -e "s#$RE#\3#"`
        VERSION="v$MAJOR.$MINOR.`expr $PATCH + 1`$TAIL"
    fi
fi

echo $VERSION