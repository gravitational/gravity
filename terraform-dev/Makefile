.PHONY: all
all: apply install

.PHONY: setup
setup:
	@if [ ! -f "./ssh/key" ]; then \
	ssh-keygen -t rsa -b 4096 -C "test@localhost" -N "" -f ./ssh/key; \
	fi;
	@if [ ! -f "/var/lib/libvirt/images/ubuntu-16.04-server-cloudimg-amd64-disk1.img" ]; then \
	curl -o /var/lib/libvirt/images/ubuntu-16.04-server-cloudimg-amd64-disk1.img -L https://cloud-images.ubuntu.com/releases/xenial/release/ubuntu-16.04-server-cloudimg-amd64-disk1.img; \
	fi;

# plan runs terraform plan on a local libvirt cluster
.PHONY: plan
plan: setup
	terraform init
	terraform plan

# apply runs terraform apply on a local libvirt cluster
.PHONY: apply
apply: setup
	terraform init
	terraform apply -auto-approve

# install installs telekube on a machine
.PHONY: install
install:
	ansible-playbook -v install.yaml

# install installs wireguard
.PHONY: install-wireguard
install-wireguard:
	ansible-playbook -v install-wireguard.yaml

# install installs wireguard
.PHONY: install-wireguard-cni
install-wireguard-cni:
	ansible-playbook -v install-wireguard-cni.yaml

# update-gravity gravity updates gravity binary and k8s service
.PHONY: update-gravity
update-gravity:
	ansible-playbook -v update-gravity.yaml

# Update gravity updates planet binary
.PHONY: update-planet-bin
update-planet-bin:
	ansible-playbook -v update-planet-bin.yaml

# start starts VMs after OS reboot
.PHONY: start
start:
	virsh net-start vm_network
	virsh start telekube1

# destroy destroys a cluster
.PHONY: destroy
destroy:
	terraform destroy -auto-approve

# ssh connects to a local VM node via SSH
.PHONY: ssh0
ssh0:
	ssh -F ssh/config ubuntu@172.28.128.3

# ssh connects to a local VM node via SSH
.PHONY: ssh1
ssh1:
	ssh -F ssh/config ubuntu@172.28.128.4

# ssh connects to a local VM node via SSH
.PHONY: ssh2
ssh2:
	ssh -F ssh/config ubuntu@172.28.128.5

