IMAGEURL := https://cloud-images.ubuntu.com/releases/bionic/release/ubuntu-18.04-server-cloudimg-amd64.img
CPU_COUNT ?= 1
ROOT_DISK_SIZE ?= 15000000000
DATA_DISK_SIZE ?= 15000000000
OS ?= ubuntu
ANSIBLE_EXTRA_VARS ?=
NODES_COUNT ?= 3

ifeq ($(OS), centos)
	IMAGEURL=https://cloud.centos.org/centos/7/images/CentOS-7-x86_64-GenericCloud-1907.qcow2
endif

IMAGENAME := $(shell basename $(IMAGEURL))
TF_VARIABLES := TF_VAR_image_name=$(IMAGENAME) TF_VAR_cpu_count=$(CPU_COUNT)\
	TF_VAR_root_disk_size=$(ROOT_DISK_SIZE) \
	TF_VAR_data_disk_size=$(DATA_DISK_SIZE) \
	TF_VAR_username=$(OS) TF_VAR_nodes_count=$(NODES_COUNT)

.PHONY: all
all: apply install

.PHONY: setup
setup:
	@if [ ! -f "./ssh/key" ]; then \
	ssh-keygen -t rsa -b 4096 -C "test@localhost" -N "" -f ./ssh/key; \
	fi;
	@if [ ! -f "/var/lib/libvirt/images/$(IMAGENAME)" ]; then \
	sudo curl -o /var/lib/libvirt/images/$(IMAGENAME) -L $(IMAGEURL); \
	fi;

# plan runs terraform plan on a local libvirt cluster
.PHONY: plan
plan: setup
	$(TF_VARIABLES) terraform init
	$(TF_VARIABLES) terraform plan

# apply runs terraform apply on a local libvirt cluster
.PHONY: apply
apply: setup
	$(TF_VARIABLES) terraform init
	$(TF_VARIABLES) terraform apply -auto-approve

# install installs telekube on a machine
.PHONY: install
install:
	ansible-playbook -v install.yaml $(ANSIBLE_EXTRA_VARS)

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
	ssh -F ssh/config $(OS)@172.28.128.3

# ssh connects to a local VM node via SSH
.PHONY: ssh1
ssh1:
	ssh -F ssh/config $(OS)@172.28.128.4

# ssh connects to a local VM node via SSH
.PHONY: ssh2
ssh2:
	ssh -F ssh/config $(OS)@172.28.128.5
