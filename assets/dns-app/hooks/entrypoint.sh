#!/bin/sh
set -e

echo "Assuming changeset from the environment: $RIG_CHANGESET"
if [ $1 = "update" ]; then
    echo "Deleting old resources"
    rig delete ds/kube-dns-v18 --resource-namespace=kube-system --force --debug
    rig delete ds/kube-dns --resource-namespace=kube-system --force --debug
    rig delete serviceaccount/kube-dns --resource-namespace=kube-system --force --debug

    echo "Creating new resources"
    rig upsert -f /var/lib/gravity/resources/dns.yaml

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
else
    echo "Missing argument, should be either 'update' or 'rollback'"
fi
