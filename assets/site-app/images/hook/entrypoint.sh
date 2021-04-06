#!/bin/sh
set -ex

echo "Assuming changeset from the environment: $RIG_CHANGESET"
# note that rig does not take explicit changeset ID
# taking it from the environment variables
if [ $1 = "update" ]; then
    echo "Checking: $RIG_CHANGESET"
    if rig status $RIG_CHANGESET --retry-attempts=1 --retry-period=1s --quiet; then exit 0; fi

    echo "Starting update, changeset: $RIG_CHANGESET"
    rig cs delete --force -c cs/$RIG_CHANGESET

    echo "Deleting old configmaps/gravity-site"
    rig delete configmaps/gravity-site --resource-namespace=kube-system --force

    echo "Creating or updating configmap"
    rig configmap gravity-site --resource-namespace=kube-system --from-file=/var/lib/gravity/resources/config
    if [ -n "$MANUAL_UPDATE" ]; then
        rig upsert -f /var/lib/gravity/resources/site.yaml --debug
        if kubectl get namespaces/monitoring > /dev/null 2>&1; then
            rig upsert -f /var/lib/gravity/resources/monitoring.yaml --debug
        fi
    fi

    # Check to see if the Ops Center ConfigMap has already been created.
    # Only relevant for Ops Center mode.
    # TODO: Only create/update Ops Center configmap when gravity-site is
    # running in Ops Center mode
    if ! kubectl get configmap/gravity-opscenter --namespace=kube-system > /dev/null 2>&1; then
      echo "Creating opscenter configmap"
      kubectl apply -f /var/lib/gravity/resources/opscenter.yaml
    fi

    echo "Checking status"
    rig status $RIG_CHANGESET --retry-attempts=120 --retry-period=1s --debug

    echo "Freezing"
    rig freeze
elif [ $1 = "rollback" ]; then
    echo "Reverting changeset $RIG_CHANGESET"
    rig revert
    rig cs delete --force -c cs/$RIG_CHANGESET
else
    echo "Missing argument, should be either 'update' or 'rollback'"
fi
