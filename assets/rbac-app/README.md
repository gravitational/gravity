# RBAC

This app sets up Kubernetes [RBAC](https://kubernetes.io/docs/admin/authorization)
and sets up several built-in roles and mappings:

* Default service account in kube-system namespace is cluster-admin
* `view` group can view, but can't change anything
* `admin` group is a cluster admin
* `edit` group is a power cluster user without admin privileges


