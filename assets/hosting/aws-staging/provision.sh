#!/bin/bash

CENTOS7="ami-d440a6e7"
SERVERS="master worker-a worker-b db"
DOMAIN="s.gravitational.io"
PORT=61822

# copies all *.pub keys from 'keys' folder (create it) into all servers
copy_keys() {
    if [ -d keys ]; then
        for server in ${SERVERS} ; do
            for key in $(find keys/ -type f -name "*.pub") ; do
                echo copying $key to ${server}.s.gravitational.io
                cat ${key} | ssh centos@${server}.${DOMAIN} "cat - >> .ssh/authorized_keys"
            done
        done
    fi
}

# installs packges on all servers
install_apps() {
    for server in ${SERVERS} ; do
        ssh -t -p $PORT centos@${server}.${DOMAIN} "sudo yum install -y epel-release"
        ssh -t -p $PORT centos@${server}.${DOMAIN} "sudo yum install -y htop vim tree screen"
        ssh -t -p $PORT centos@${server}.${DOMAIN} "sudo yum remove -y postfix"
    done
}

# reboots all servers
reboot() {
    for server in ${SERVERS} ; do
        ssh -t -p $PORT centos@${server}.${DOMAIN} "sudo reboot"
    done
}

# basic server config for all: 
#    - move SSH port to 61822 
#    - enable passwordless sudo without TTY
#    - drop a nicer .bashrc
config_env() {
    for server in ${SERVERS} ; do
        scp bashrc centos@${server}.${DOMAIN}:.bashrc
        cat sudoers | ssh centos@${server}.${DOMAIN} "sudo tee /etc/sudoers > /dev/null"
        ssh -t centos@${server}.${DOMAIN} "sudo semanage port -a -t ssh_port_t -p tcp 61822"
        cat sshd_config | ssh centos@${server}.${DOMAIN} "sudo tee /etc/ssh/sshd_config > /dev/null"
    done
}


# mounts local SSD to /dev/gravity. run this _after_ config_env() has succeeded
prep_storage() {
    for server in ${SERVERS} ; do
        echo "Preparing /var/lib/gravity on ${server}"
        ssh -p $PORT centos@${server}.${DOMAIN} "sudo rm -rf /var/lib/gravity"
        ssh -p $PORT centos@${server}.${DOMAIN} "sudo mkdir -p /mnt/gravity;sudo chown centos:centos /mnt/gravity"
        ssh -p $PORT centos@${server}.${DOMAIN} "sudo ln -s /mnt/gravity /var/"
    done
}

# creates a single AWS instace. run it X times and update $SERVERS variable above after that.
make_instance() {
    aws ec2 run-instances \
        --count 1 \
        --instance-type m3.large \
        --key-name ekontsevoy \
        --image-id ${CENTOS7} \
        --security-group-ids sg-a0556ec4 \
        --subnet-id subnet-66ee5611  \
        --block-device-mappings "[{\"DeviceName\":\"/dev/xvdb\",  \"VirtualName\":\"ephemeral0\"}]"
}

# tags instances with 'staging' tag
tag_instances() {
    aws ec2 create-tags --resources i-aa266173 i-a9266170 i-d0266109 i-a8266171 --tags Key=env,Value=staging
}


#
# call this script with a function name as an argument, 
# like:
#
# ./provision.sh install_apps
#
$1
