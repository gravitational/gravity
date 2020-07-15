#!/bin/sh
set -eu

#
# update the ops/pack advertise IPs so they are IPs of the master server we're running on
#
export AGENT_SERVER_ADVERTISE_ADDR=$POD_IP:3008
export ADVERTISE_ADDR=$POD_IP:3009
PROFILE="${PROFILE:=}"
DEBUG="${DEBUG:=}"

#
# start gravity cluster controller
#
/opt/gravity/gravity site start ${DEBUG} --httpprofile=${PROFILE} /opt/gravity/config --init-from /opt/gravity-import
