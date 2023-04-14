ARCH=`uname -m`
case $ARCH in
    aarch64)
        ARCH="arm64"
    ;;
    x86_64)
        ARCH="amd64"
    ;;
esac

echo $ARCH
