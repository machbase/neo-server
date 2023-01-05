
if [ ! -d ../packages ]; then 
    mkdir ../packages
fi

VMS=( "utm-arm-ubuntu18" "utm-amd-ubuntu18" )

for VM in  ${VMS[@]}; do
    ssh $VM 'bash -s' <  rbuild-remote.sh && \
    scp $VM:/tmp/machbase-neo-v*.zip ../packages
done
