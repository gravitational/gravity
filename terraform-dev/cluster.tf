variable "image_name" {
  type = "string"
  default = "ubuntu-16.04-server-cloudimg-amd64-disk1.img"
}

variable "gravity_dir_size" {
  type = "string"
  default = "12000000000"
}

variable "memory_size" {
  type = "string"
  default = "4096"
}

variable "cpu_count" {
  type = "string"
  default = "1"
}

# Initialize the provider
provider "libvirt" {
  uri = "qemu:///system"
}

# Use locally pre-fetched image
resource "libvirt_volume" "os-qcow2" {
  name = "os-${count.index}-qcow2"
  pool = "default"
  source = "/var/lib/libvirt/images/${var.image_name}"
  format = "qcow2"
  count = 3
}

# Create a network for our VMs
resource "libvirt_network" "vm_network" {
   name = "vm_network"
   addresses = ["172.28.128.0/24"]
}

# 12 GB volume for gravity install
resource "libvirt_volume" "gravity" {
  name = "gravity-disk-${count.index}.qcow2"
  pool = "default"
  size = "${var.gravity_dir_size}"
  count = 3
}

# 12 GB volume for tmp
resource "libvirt_volume" "tmp" {
  name = "gravity-tmp-disk-${count.index}.qcow2"
  pool = "default"
  size = 12000000000
  count = 3
}

# Use CloudInit to add our ssh-key to the instance
resource "libvirt_cloudinit" "commoninit" {
  name           = "commoninit.iso"
  user_data = <<EOF
    #cloud-config
    packages: [python, curl, htop, iotop, lsof, ltrace, mc, net-tools, strace, tcpdump, telnet, vim, wget, ntp, traceroute, bash-completion]
    ssh_authorized_keys: ["${file("ssh/key.pub")}"]
    write_files:
    - content: "br_netfilter"
      path: /etc/modules-load.d/br_netfilter.conf
    - content: "ebtables"
      path: /etc/modules-load.d/ebtables.conf
    - content: "overlay"
      path: /etc/modules-load.d/overlay.conf
    - content: |
        ip_tables
        iptable_nat
        iptable_filter
      path: /etc/modules-load.d/iptables.conf
    - content: |
        net.bridge.bridge-nf-call-arptables=1
        net.bridge.bridge-nf-call-ip6tables=1
        net.bridge.bridge-nf-call-iptables=1
      path: /etc/sysctl.d/10-br-netfilter.conf
    - content: |
        net.ipv4.ip_forward=1
      path: /etc/sysctl.d/10-ipv4-forwarding-on.conf
    - content: |
        fs.may_detach_mounts=1
      path: /etc/sysctl.d/10-fs-may-detach-mounts.conf
    runcmd:
    - 'modprobe overlay'
    - 'modprobe br_netfilter'
    - 'modprobe ebtables'
    - 'modprobe ip_tables'
    - 'modprobe iptable_nat'
    - 'modprobe iptable_filter'
    - 'sysctl -p /etc/sysctl.d/10-br-netfilter.conf'
    - 'sysctl -p /etc/sysctl.d/10-ipv4-forwarding-on.conf'
    - 'sysctl -p /etc/sysctl.d/10-fs-may-detach-mounts.conf'
    - 'parted -a opt /dev/vdb mktable msdos'
    - 'parted -a opt /dev/vdb mkpart primary ext4 0% 100%'
    - 'mkfs.ext4 -L GRAVITY /dev/vdb1'
    - 'parted -a opt /dev/vdc mktable msdos'
    - 'parted -a opt /dev/vdc mkpart primary ext4 0% 100%'
    - 'mkfs.ext4 -L TMP /dev/vdc1'
    - 'echo "/dev/vdb1 /var/lib/gravity ext4 discard,noatime,nodiratime 0 0" >> /etc/fstab'
    - 'echo "/dev/vdc1 /tmp ext4 discard,noatime,nodiratime 0 0" >> /etc/fstab'
    - 'mkdir -p /var/lib/gravity'
    - 'mount -a'
    - 'chmod 777 /tmp'
    EOF
}

# Create the machine
resource "libvirt_domain" "domain-gravity" {
  name = "telekube${count.index}"
  memory = "${var.memory_size}"
  vcpu = "${var.cpu_count}"
  count = 3
  cloudinit = "${libvirt_cloudinit.commoninit.id}"

  network_interface {
    hostname = "telekube${count.index}"
    network_id = "${libvirt_network.vm_network.id}"
    addresses = ["172.28.128.${count.index+3}"]
    mac = "6E:02:C0:21:62:5${count.index+3}"
  }

  # IMPORTANT
  # Ubuntu can hang if an isa-serial is not present at boot time.
  # If you find your CPU 100% and never is available this is why
  console {
    type        = "pty"
    target_port = "0"
    target_type = "serial"
  }

  console {
    type        = "pty"
    target_type = "virtio"
    target_port = "1"
  }

  disk {
    volume_id = "${element(libvirt_volume.os-qcow2.*.id, count.index)}"
  }

  disk {
    volume_id = "${element(libvirt_volume.gravity.*.id, count.index)}"
  }

  disk {
    volume_id = "${element(libvirt_volume.tmp.*.id, count.index)}"
  }
}
