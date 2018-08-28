#!/bin/sh

#
# update the ops/pack advertise IPs so they are IPs of the master server we're running on
#
export AGENT_SERVER_ADVERTISE_ADDR=$POD_IP:3008
export ADVERTISE_ADDR=$POD_IP:3009

#
# start gravity site
#
/opt/gravity/gravity --debug --httpprofile=localhost:6060 site start /opt/gravity/config --init-from /opt/gravity-import
