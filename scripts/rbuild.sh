
if [ ! -d ../packages ]; then 
    mkdir ../packages
fi

# Build linux-arm64

ssh utm-arm-ubuntu18 'bash -s' <  rbuild-remote.sh && \
scp utm-arm-ubuntu18:/tmp/machsvr-v*.zip ../packages