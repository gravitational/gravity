#!/usr/bin/make -f

SHELL := /bin/bash

ifneq ("$(wildcard vars.mk)","")
include vars.mk
endif

CWD := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

ANSIBLE_CONFIG := $(CWD)/ansible/ansible.cfg
ANSIBLE_FLAGS ?= -vv
APIKEY ?=
APP ?= telekube
FLAVOR ?= three
REMOTE_HOSTS ?=
REMOTE_KEY ?=
REMOTE_SSH_CONFIG ?= 
export

.PHONY: 1bignode
1bignode:
	export VAGRANT_NODE_RAM=$${VAGRANT_NODE_ARM:-16000} && \
	export VAGRANT_DEVICE_SIZE=$${VAGRANT_DEVICE_SIZE:-100} && \
	export VAGRANT_DEVICE_NUMBER=$${VAGRANT_DEVICE_NUMBER:-2} && \
	export VAGRANT_INSTANCES=$${VAGRANT_INSTANCES:-1} && \
	vagrant up

.PHONY: 1node
1node:
	export VAGRANT_INSTANCES=1  && \
	vagrant up

.PHONY: 3nodes
3nodes:
	export VAGRANT_INSTANCES=3 && \
	vagrant up

.PHONY: 6nodes
6nodes:
	export VAGRANT_INSTANCES=6 && \
	vagrant up

.PHONY: destroy
destroy: clean

.PHONY: clean
clean:
	vagrant destroy -f

.PHONY: ansible-inventory
ansible-inventory:
	vagrant ssh-config | tee '$(CWD)/ansible/ssh-config'
	machines=($$(vagrant --machine-readable status | cut -d ',' -f 2 | sort -u)) && \
	( \
		echo '[local]' && \
		echo 'localhost ansible_connection=local' && \
		echo '[nodes]' && \
		for (( i=0 ; i<$${#machines[@]}; i++ )); do \
			echo "$${machines[i]}" ; \
		done ; \
		echo '[first]' && \
		echo "$${machines[0]}" ; \
		echo '[all:vars]' && \
		echo 'apikey=$(APIKEY)' && \
		echo 'app=$(APP)' && \
		echo 'flavor=$(FLAVOR)' ; \
	) | tee '$(CWD)/ansible/inventory'

.PHONY: ansible-%
ansible-%: ansible-inventory
	ansible-playbook \
		$(ANSIBLE_FLAGS) \
		--ssh-extra-args='-F $(CWD)/ansible/ssh-config' \
		-i '$(CWD)/ansible/inventory' \
		$(CWD)/ansible/$*.yaml

.PHONY: remote-inventory
remote-inventory:
	@if [ -z "$(REMOTE_HOSTS)" ]; then \
	  echo "REMOTE_HOSTS is not set use space delimited targets to deploy"; exit 1; \
	fi;
	@if [ -z "$(REMOTE_KEY)" ]; then \
	  echo "REMOTE_KEY is not set, use path to SSH key file"; exit 1; \
	fi;
	@if [ -z "$(REMOTE_USER)" ]; then \
	  echo "REMOTE_USER is not set, use SSH user"; exit 1; \
	fi;
	@if [ -z "$(REMOTE_SSH_CONFIG)" ]; then \
	( \
		echo 'Host *' && \
		echo ' IdentityFile $(REMOTE_KEY)' && \
		echo ' User $(REMOTE_USER)' && \
		echo '' ; \
	) | tee '$(CWD)/ansible/ssh-config'; \
    else \
    cp $(REMOTE_SSH_CONFIG) $(CWD)/ansible/ssh-config; \
    fi;
	( \
		echo '[local]' && \
		echo 'localhost ansible_connection=local' && \
		echo '[nodes]' && \
		export REMOTE_HOSTS=( $(REMOTE_HOSTS) ) && \
		for (( i=0 ; i<$${#REMOTE_HOSTS[@]}; i++ )); do \
			echo "$${REMOTE_HOSTS[i]}" ; \
		done ; \
		echo '[first]' && \
		echo "$${REMOTE_HOSTS[0]}" ; \
		echo '[all:vars]' && \
		echo 'apikey=$(APIKEY)' && \
		echo 'app=$(APP)' && \
		echo 'flavor=$(FLAVOR)' ; \
	) | tee '$(CWD)/ansible/inventory'


.PHONY: remote-%
remote-%: remote-inventory
	ANSIBLE_CONFIG= ansible-playbook \
		$(ANSIBLE_FLAGS) \
		--ssh-extra-args='-F $(CWD)/ansible/ssh-config -i $(REMOTE_KEY) -l $(REMOTE_USER)' \
		-i '$(CWD)/ansible/inventory' \
		$(CWD)/ansible/$*.yaml

