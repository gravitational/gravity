variable "access_key" {}
variable "secret_key" {}
variable "region" {
  default = "us-east-1"
}
variable "key_pair" {
  default = "ops"
}
variable "cluster_name" {}
variable "nodes" {
  default = 3
}
variable "instance_type" {
  default = "c3.2xlarge"
}
variable "installer_url" {}

provider "aws" {
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region = "${var.region}"
}

output "installer_ip" {
  value = "${aws_instance.installer_node.public_ip}"
}

output "private_ips" {
  value = "${aws_instance.installer_node.private_ip} ${join(" ", aws_instance.node.*.private_ip)}"
}

output "public_ips" {
  value = "${aws_instance.installer_node.public_ip} ${join(" ", aws_instance.node.*.public_ip)}"
}

resource "aws_placement_group" "cluster" {
  name = "${var.cluster_name}"
  strategy = "cluster"
}

# ALL UDP and TCP traffic is allowed within the security group
resource "aws_security_group" "cluster" {
    tags {
        Name = "${var.cluster_name}"
    }

    # Admin gravity site for testing
    ingress {
        from_port   = 32009
        to_port     = 32009
        protocol    = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }

    # SSH access from anywhere
    ingress {
        from_port   = 22
        to_port     = 22
        protocol    = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }

    # install wizard
    ingress {
        from_port   = 61009
        to_port     = 61009
        protocol    = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }

    ingress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        self = true
    }

    egress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }
}

resource "aws_instance" "installer_node" {
    ami = "ami-366be821"
    instance_type = "${var.instance_type}"
    associate_public_ip_address = true
    source_dest_check = "false"
    ebs_optimized = true
    security_groups = ["${aws_security_group.cluster.name}"]
    key_name = "${var.key_pair}"
    placement_group = "${aws_placement_group.cluster.id}"
    count = "1"

    tags {
        Name = "${var.cluster_name}"
    }

    user_data = <<EOF
#!/bin/bash
set -euo pipefail

umount /dev/xvdb || true
umount /dev/xvdc || true
mkfs.ext4 /dev/xvdb
mkfs.ext4 /dev/xvdc
mkfs.ext4 /dev/xvdf

sed -i.bak '/xvdb/d' /etc/fstab
sed -i.bak '/xvdc/d' /etc/fstab
echo -e '/dev/xvdb\t/var/lib/gravity\text4\tdefaults\t0\t2' >> /etc/fstab
echo -e '/dev/xvdc\t/var/lib/data\text4\tdefaults\t0\t2' >> /etc/fstab
echo -e '/dev/xvdf\t/var/lib/gravity/planet/etcd\text4\tdefaults\t0\t2' >> /etc/fstab

mkdir -p /var/lib/gravity /var/lib/data
mount /var/lib/gravity
mount /var/lib/data
mkdir -p /var/lib/gravity/planet/etcd
mount /var/lib/gravity/planet/etcd
chown -R 1000:1000 /var/lib/gravity /var/lib/data /var/lib/gravity/planet/etcd
sed -i.bak 's/Defaults    requiretty/#Defaults    requiretty/g' /etc/sudoers

sudo -u centos mkdir -p /home/centos/installer
yum install -y python; sudo easy_install awscli
sudo -u centos AWS_ACCESS_KEY_ID=${var.access_key} AWS_SECRET_ACCESS_KEY=${var.secret_key} aws s3 cp ${var.installer_url} /home/centos/installer.tar.gz
EOF

    root_block_device {
        delete_on_termination = true
        volume_type = "io1"
        volume_size = "50"
        iops = 500
    }

    # /var/lib/gravity device
    ephemeral_block_device = {
        virtual_name = "ephemeral0"
        device_name = "/dev/xvdb"
    }

    # /var/lib/data device
    ephemeral_block_device = {
        virtual_name = "ephemeral1"
        device_name = "/dev/xvdc"
    }

    # gravity/docker data device
    ebs_block_device = {
        volume_size = "100"
        volume_type = "io1"
        device_name = "/dev/xvde"
        iops = 3000
        delete_on_termination = true
    }

    # etcd device
    ebs_block_device = {
        volume_size = "100"
        volume_type = "io1"
        device_name = "/dev/xvdf"
        iops = 3000
        delete_on_termination = true
    }
}

resource "aws_instance" "node" {
    ami = "ami-366be821"
    instance_type = "${var.instance_type}"
    associate_public_ip_address = true
    source_dest_check = "false"
    ebs_optimized = true
    security_groups = ["${aws_security_group.cluster.name}"]
    key_name = "${var.key_pair}"
    placement_group = "${aws_placement_group.cluster.id}"
    count = "${var.nodes - 1}"

    tags {
        Name = "${var.cluster_name}"
    }

    user_data = <<EOF
#!/bin/bash
set -euo pipefail

umount /dev/xvdb || true
umount /dev/xvdc || true
mkfs.ext4 /dev/xvdb
mkfs.ext4 /dev/xvdc
mkfs.ext4 /dev/xvdf

sed -i.bak '/xvdb/d' /etc/fstab
sed -i.bak '/xvdc/d' /etc/fstab
echo -e '/dev/xvdb\t/var/lib/gravity\text4\tdefaults\t0\t2' >> /etc/fstab
echo -e '/dev/xvdc\t/var/lib/data\text4\tdefaults\t0\t2' >> /etc/fstab
echo -e '/dev/xvdf\t/var/lib/gravity/planet/etcd\text4\tdefaults\t0\t2' >> /etc/fstab

mkdir -p /var/lib/gravity /var/lib/data
mount /var/lib/gravity
mount /var/lib/data
mkdir -p /var/lib/gravity/planet/etcd
mount /var/lib/gravity/planet/etcd
chown -R 1000:1000 /var/lib/gravity /var/lib/data /var/lib/gravity/planet/etcd
sed -i.bak 's/Defaults    requiretty/#Defaults    requiretty/g' /etc/sudoers
EOF

    root_block_device {
        delete_on_termination = true
        volume_type = "io1"
        volume_size = "50"
        iops = 500
    }

    # /var/lib/gravity device
    ephemeral_block_device = {
        virtual_name = "ephemeral0"
        device_name = "/dev/xvdb"
    }

    # /var/lib/data device
    ephemeral_block_device = {
        virtual_name = "ephemeral1"
        device_name = "/dev/xvdc"
    }

    # gravity/docker data device
    ebs_block_device = {
        volume_size = "100"
        volume_type = "io1"
        device_name = "/dev/xvde"
        iops = 3000
        delete_on_termination = true
    }

    # etcd device
    ebs_block_device = {
        volume_size = "100"
        volume_type = "io1"
        device_name = "/dev/xvdf"
        iops = 3000
        delete_on_termination = true
    }
}
