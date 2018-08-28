#!/bin/sh
set -e

#
# update the ops/pack advertise IPs so they are IPs of the master server we're running on
#
export AGENT_SERVER_ADVERTISE_ADDR=$POD_IP:3008
export ADVERTISE_ADDR=$POD_IP:3009

#
# are we running in dev mode?
#
sed -i -E "s/devmode:.+/devmode: $DEVMODE/g" /var/lib/gravity/resources/config/gravity.yaml

#
# import cluster state
#
/usr/bin/dumb-init /opt/gravity/gravity --debug site init /var/lib/gravity/resources/config --init-from /opt/gravity-import

#
# create all resources
#

# create opscenter empty config, ignore if already exists
/usr/local/bin/kubectl create -f /var/lib/gravity/resources/opscenter.yaml || true
# create config from directory
if ! /usr/local/bin/kubectl get configmap/gravity-cluster --namespace=kube-system > /dev/null 2>&1; then
    echo "Creating cluster configmap"
    /usr/local/bin/kubectl create configmap gravity-site --namespace=kube-system --from-file=/var/lib/gravity/resources/config
fi

# create daemon set with app
/usr/local/bin/kubectl apply -f /var/lib/gravity/resources/site.yaml
