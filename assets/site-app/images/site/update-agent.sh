#!/bin/bash
set -e

# update the ops/package service advertise IPs to point to actual Pods
export AGENT_SERVER_ADVERTISE_ADDR=$POD_IP:3008
export ADVERTISE_ADDR=$POD_IP:3009

/opt/gravity/gravity --debug --httpprofile=localhost:6060 update resume /opt/gravity/config --init-from /opt/gravity-import
