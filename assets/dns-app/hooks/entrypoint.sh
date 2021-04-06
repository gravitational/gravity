#!/bin/sh
set -ex

echo "Assuming changeset from the environment: $RIG_CHANGESET"
if [ $1 = "update" ]; then
    echo "Updating resources"
    rig upsert -f /var/lib/gravity/resources/dns.yaml

    echo "Deleting coredns daemonset that has been replaced by a deployment"
    rig delete ds/coredns-worker --resource-namespace=kube-system --force

    echo "Checking status"
    rig status $RIG_CHANGESET --retry-attempts=600 --retry-period=1s --debug
    echo "Freezing"
    rig freeze
elif [ $1 = "rollback" ]; then
    echo "Reverting changeset $RIG_CHANGESET"
    rig revert
    rig cs delete --force -c cs/$RIG_CHANGESET
elif [ $1 = "install" ]; then
    echo "Creating new resources"
    rig upsert -f /var/lib/gravity/resources/dns.yaml
    echo "Freezing"
    rig freeze
else
    echo "Missing argument, should be either 'update' or 'rollback'"
fi
