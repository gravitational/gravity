variable "image_name" {
  type = string
  default = "ubuntu-18.04-server-cloudimg-amd64.img"
}

variable "disk_size" {
  type = string
  default = "15000000000"
}

variable "memory_size" {
  type = string
  default = "4096"
}

variable "cpu_count" {
  type = string
  default = "1"
}

variable "nodes_count" {
  type = string
  default = "3"
}

# Initialize the provider
provider "libvirt" {
  uri = "qemu:///system"
}

# Use locally pre-fetched image
resource "libvirt_volume" "os-qcow2" {
  name = "os-disk-${count.index}.qcow2"
  pool = "default"
  source = "/var/lib/libvirt/images/${var.image_name}"
  format = "raw"
  count = var.nodes_count
}

# Create a network for our VMs
resource "libvirt_network" "vm_network" {
   name = "vm_network"
   addresses = ["172.28.128.0/24"]
   dns {
     enabled = true
     local_only = false
   }
}

resource "libvirt_volume" "root" {
  name = "root-disk-${count.index}.qcow2"
  base_volume_id = element(libvirt_volume.os-qcow2.*.id, count.index)
  pool = "default"
  size = var.disk_size
  count = var.nodes_count
}

# Use CloudInit to add our ssh-key to the instance
resource "libvirt_cloudinit_disk" "commoninit" {
  name      = "commoninit-${count.index}.iso"
  user_data = templatefile("cloudinit.cfg", {
            ip_address = "172.28.128.${count.index+3}",
            hostname = "telekube${count.index}"
            })
  count     = var.nodes_count
}

# Create the machine
resource "libvirt_domain" "domain-gravity" {
  name = "telekube${count.index}"
  memory = var.memory_size
  vcpu = var.cpu_count
  count = var.nodes_count
  cloudinit = element(libvirt_cloudinit_disk.commoninit.*.id, count.index)

  network_interface {
    hostname = "telekube${count.index}"
    network_id = libvirt_network.vm_network.id
    addresses = ["172.28.128.${count.index+3}"]
    mac = "6E:02:C0:21:62:5${count.index+3}"
    wait_for_lease = true
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
    volume_id = element(libvirt_volume.root.*.id, count.index)
  }
}

terraform {
  required_version = ">= 0.12"
}
