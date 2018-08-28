#!/bin/sh
set -e

#
# delete only the service to remove the provisioned load balancer but do not touch
# gravity site because teleports still use it as an auth server
#
/usr/local/bin/kubectl delete services/gravity-site --namespace=kube-system --ignore-not-found

#
# removing a load balancer on AWS happens in background (even if control panel
# shows it as deleted) and may take some time
#
sleep 120
