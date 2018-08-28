# Rig

Rig tool uses third-party resources to add changeset semantics to Kubernetes operations:

```sh
# upsert a daemon set (changeset is created automatically)
rig upsert -c change1 -f daemonset.yaml
# delete a service
rig delete -c change1 svc/service1
# update config map
rig delete -c change1 configmaps/cf1
# check status
rig status -c change1
# revert everything
rig revert -c change1
# or freeze changeset, so it can no longer be updated
rig freeze -c change1
```

**Environment variables**

Use environment variables to bind rig to particular changeset

```sh
export RIG_CHANGESET=cs1
rig upsert -f daemonset.yaml
```

## Changeset management

You can view your changesets to see what happened:

**View all changesets**

```sh
rig get
```

**Get detailed operations log for a changeset**

```sh
rig get change1
rig get change1 -o yaml
```

**Delete a changeset**

```sh
rig cs delete change1
```

## Details

### Supported resources

The following resources are supported:

* ConfigMap
* DaemonSet
* ReplicationController
* Service
* Secret
* Deployment

### Updates

Only `Deployment` updates are rolling, updating of daemon sets or replication controllers simply deletes all the pods

### Status checks

Rig adds additional condition to consider `Deployment`, `DaemonSet` or `ReplicatonController` as ready - all pods must be in `Running` state.



