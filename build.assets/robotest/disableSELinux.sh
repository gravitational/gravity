#!/bin/sh
setenforce Permissive
sed -i s/^SELINUX=.*$/SELINUX=permissive/ /etc/selinux/config
sestatus
