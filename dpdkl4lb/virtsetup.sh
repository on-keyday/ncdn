#  curl https://releases.ubuntu.com/24.04.2/ubuntu-24.04.2-live-server-amd64.iso -o /var/lib/libvirt/images/ubuntu-server.iso
# qemu-img create -f qcow2 /var/lib/libvirt/images/dpdk_guest.qcow2 20G
virt-install \
--name dpdk-guest \
--memory 4096 \
--vcpus 2 \
--disk path=/var/lib/libvirt/images/dpdk_guest.qcow2,format=qcow2 \
--location /var/lib/libvirt/images/ubuntu-server.iso,kernel=casper/vmlinuz,initrd=casper/initrd \
--network network=default,model=virtio \
--os-variant ubuntu24.04 \
--graphics none \
--console pty,target_type=serial  \
--extra-args console=ttyS0,115200n8  
