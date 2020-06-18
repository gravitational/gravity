# Cluster Management

This chapter covers Telekube Cluster administration.

Every application packaged with Telekube ("Application Bundle") is a self-contained Kubernetes system.
This means that every application running on a cluster ("Telekube Cluster" or "Cluster") consists
of the following components:

1. The application and its services: workers, databases, caches, etc.
2. Kubernetes services and CLI tools.
3. Telekube tooling such as the [Teleport SSH server](https://gravitational.com/teleport)
   and the Gravity CLI.

Telekube uses software we call "Gravity" for managing Clusters. The Gravity CLI is
the Telekube interface that can be used to manage the Cluster. Each Telekube Cluster
also has a graphical, web-based UI to view and manage the Cluster.

You can also use familiar Kubernetes tools such as `kubectl` to perform regular cluster
tasks such as watching logs, seeing stats for pods or volumes or managing configurations.

## Kubernetes Environment

Every Telekube Cluster is a standalone instance of Kubernetes running multiple nodes, so
the standard Kubernetes tools will work and Kubernetes rules will apply.

However, Telekube pre-configures Kubernetes to be as reliable as possible
greatly reducing the need for ongoing active management. This includes the following
default configurations:

* All Kubernetes services such as `kubelet` or `kube-apiserver` always run
	enclosed inside Telekube's own Gravity container. This makes it possible for
	Telekube to monitor Kubernetes' health and perform cluster updates.

* Telekube always configures Kubernetes to run in a highly available ("HA")
	configuration. This means that any node can go down without disrupting
	Kubernetes' operation.

* Telekube runs its own local Docker registry which is used as a
	cluster-level cache for container images. This makes application updates
	and restarts faster and more reliable.

* Telekube provides the ability to perform cluster state snapshots as part of cluster
	updates or to be used independently.

### Kubernetes Extensions

Gravitational also offers various extensions for Kubernets that can be
pre-packaged and distributed with your application. Examples of such solutions
include:

* Master-slave controllers for popular open source databases.
* On-site emulators of popular AWS APIs such as S3 or SQS.
* Intra-cluster network encryption.
* Application monitoring and integrations with 3rd party data collection
  facilities such as Splunk.
* Identity management for integrating cluster access into existing OAuth
  providers.

!!! tip "Note"
	  The list above is not complete. Gravitational Solutions Engineering offers a
	  wide variety of Kubernetes integration and migration services. Reach out to
	  `info@gravitational.com` if you have questions.

## Gravity Tool

Gravity is the tool used to manage the cluster. Gravity is only
available inside the cluster so you have to `tsh ssh` into the cluster
to execute `gravity` commands. You can read more about `tsh` in the [remote management](manage.md) section.

The `gravity` commands allows you to:

1. Quickly add new nodes to a cluster.
2. Remove nodes from a cluster.
3. Monitor Telekube Cluster health.
3. Update / backup / restore the Telekube Cluster.

`gravity` commands can be used on any node of the cluster. Or you can execute it
remotely via an SSH tunnel by chaining it to the end of the `tsh ssh` command.

The full list of `gravity` commands:

| Command   | Description                                                        |
|-----------|--------------------------------------------------------------------|
| status    | show the status of the cluster and the application running in it   |
| update    | manage application updates on a Telekube Cluster               |
| upgrade   | start the upgrade operation for a Telekube Cluster             |
| plan      | view the upgrade operation plan                                    |
| rollback  | roll back the upgrade operation for a Telekube Cluster         |
| join      | add a new node to the cluster                                      |
| autojoin  | join the cluster using cloud provider for discovery                |
| leave     | decommission a node: execute on a node being decommissioned        |
| remove    | remove the specified node from the cluster                         |
| backup    | perform a backup of the application data in a cluster              |
| restore   | restore the application data from a backup                         |
| tunnel    | manage the SSH tunnel used for the remote assistance               |
| report    | collect cluster diagnostics into an archive                        |
| resource  | manage cluster resources                                           |


## Cluster Status

Running `gravity status` will give you the high level overview of the cluster health,
as well as the status of its individual components.

Here's an example of how to remotely log in to a cluster via `tsh` and check the
status of the cluster named "production":

```bash
$ tsh --cluster=production ssh admin@node gravity status
```

!!! tip "Reminder"
    Keep in mind that `tsh` always uses the Telekube Ops Center as an SSH proxy. This
    means the command above will work with clusters located behind
    corporate firewalls. You can read more in the [remote management](manage.md) section.

### Cluster Health Endpoint

Clusters expose an HTTP endpoint that provides system health information about
cluster nodes. Run the following command from one of the cluster nodes to query
the status:

```bash
$ curl -sk https://localhost:7575 | python -m json.tool
{
    "nodes": [
        {
            "member_status": {
                "addr": "192.168.121.245:7496",
                "name": "192_168_121_245.example",
                "status": "alive",
                "tags": {
                    "publicip": "192.168.121.245",
                    "role": "master"
                }
            },
            "name": "192_168_121_245.example",
            "probes": [
                {
                    "checker": "kube-apiserver",
                    "status": "running"
                },
                {
                    "checker": "docker",
                    "status": "running"
                },
                ...
                {
                    "checker": "etcd-healthz",
                    "status": "running"
                }
            ],
            "status": "running"
        }
    ],
    "status": "running"
}
```

This command displays the health status of each cluster node and the overall
cluster health. In a healthy cluster the status of all health probes, individual
nodes and the cluster overall is `running`, and the response HTTP status code
is `200 OK`.

If a node fails any of its health probes, the output reflects this by moving
the failed probe status to `failed` and the node and the cluster status to
`degraded`. For example, here is what the output would look like if a Docker
process crashed on a node:

```bash
$ curl -sk https://localhost:7575 | python -m json.tool
{
    "nodes": [
        {
            "member_status": {
                "addr": "192.168.121.245:7496",
                "name": "192_168_121_245.example",
                "status": "alive",
                "tags": {
                    "publicip": "192.168.121.245",
                    "role": "master"
                }
            },
            "name": "192_168_121_245.example",
            "probes": [
                {
                    "checker": "docker",
                    "error": "healthz check failed: Get http://docker/version: dial unix /var/run/docker.sock: connect: no such file or directory",
                    "status": "failed"
                },
                {
                    "checker": "kube-apiserver",
                    "status": "running"
                },
                ...
            ],
            "status": "degraded"
        }
    ],
    "status": "degraded"
}
```

In this case the response HTTP status code will be `503 Service Unavailable`.

## Application Status

Telekube provides a way to automatically monitor the application health.

To enable this, define a "status" hook in your Application Manifest (see
[Application Hooks](pack.md#application-hooks) section for more details on them). The Kubernetes
job defined by the status hook can perform application-specific checks. An example of an
application-specific status check could be querying a database, or checking that a
certain pod is running.

If defined, the status hook job will be invoked every minute. So it is recommended to keep
the hook quick and set `activeDeadlineSeconds` to less than a minute to ensure timely
termination. Here is an example status hook:

```yaml
hooks:
  status:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: status
      spec:
        activeDeadlineSeconds: 30
        template:
          metadata:
            name: status
          spec:
            restartPolicy: OnFailure
            containers:
              - name: status
                image: status:1.0.0
                # the check-status.sh script returns 0 if the application is healthy
                # and 1 otherwise
                command: ["/opt/check-status.sh"]
                volumeMounts:
                  - name: data
                    mountPath: /data
            # the status hook is a regular Kubernetes job
            volumes:
              - name: data
                hostPath:
                  path: /data
            nodeSelector:
              app: myapp
```

When the status hook fails, the cluster transitions to a "degraded" state which is reflected
in the Admin Control Panel by a red status indicator in the top bar. Degraded clusters cannot
perform certain operations. Once the application recovers and the status hook
completes successfully, the cluster automatically moves back to a "healthy" state.

## Exploring a Cluster

Any Telekube cluster can be explored using the standard Kubernetes tool, `kubectl`, which is
installed and configured on every cluster node. See the command's [overview](http://kubernetes.io/docs/user-guide/kubectl-overview/)
and a [full reference](https://kubernetes.io/docs/user-guide/kubectl/) to see what it can
do, or simply use `kubectl --help`.

Each Telekube Cluster also has a graphical UI to explore and manage the Cluster. To log into
the Cluster Admin UI you need to create an admin user. Please see the
[Custom Installer Screens](pack.md#custom-installation-screen) chapter for details on how
to enable a post-install screen that will let you create a local user.

## Updating a Cluster

Cluster updates can get quite complicated for complex cloud applications
composed of multiple micro-services. On a high level, there are two major layers
that will need periodic updating:

* The Kubernetes itself and its dependencies, like Docker. Telekube refers to this
  layer as "system software updates".
* The application(s) deployed inside, including auxiliary subsystems used for
  monitoring and logging. Special care must be taken around database
  migrations and the sequence in which various components are updated.

The Telekube update process is designed to update both layers. Here is how
Telekube update process works:

1. New versions of an application and the system binaries (container
   images) are downloaded and stored inside the cluster. This means that during the
   update, all data has already been saved locally and disruptions to external
   services, like Docker registries, will not affect the update process.

2. Telekube uses the Kubernetes [rolling update](https://kubernetes.io/docs/tasks/manage-daemon/update-daemon-set/)
   mechanism to perform the update.

3. Custom update hooks can be used to perform application specific
   actions before or after the update, such as database migrations.

An update can be triggered via the Ops Center web UI or from command line (CLI). The instructions below
describe updating via the CLI. The commands below can be executed as part of a remote update script to update
large numbers of remotely running application instances.

!!! warning "Upgrading to 4.23.0+"
    When upgrading a cluster via the Ops Center from pre-4.23.0 to 4.23.0 or
    higher, refer to [Upgrading to 4.23.0+](changelog.md#instructions-on-upgrading-to-4230)
	in the Release Notes for instructions.

### Uploading an Update

The first step to updating a cluster to a new version is to import a new Application
Bundle onto a Telekube Cluster. Telekube supports this in both online and offline environments.

#### Online Cluster Update

If cluster is connected to an Ops Center and has remote support turned on, it can download
updated Application Bundles directly from that Ops Center.

Use the `gravity update download` command to automatically discover if there are new
versions available and download them:

```bash
$ gravity update download              # Check and download any updates now.
$ gravity update download --every=12h  # Schedule automatic downloading of updates every 12 hours.
$ gravity update download --every=off  # Turn off automatic downloading of updates.
```

#### Offline Cluster Update

If a Telekube Cluster is offline or not connected to an Ops Center, the new version of the Application
Bundle has to be copied to one of the Application Cluster nodes and the Cluster nodes need to be accessible
to each other. To upload the new version, extract the tarball and launch the `upload` script.

### Performing Upgrade

Once a new Application Bundle has been uploaded into the Cluster, it can be upgraded using
the automatic or manual upgrade modes.

#### Automatic Upgrade Mode

An automated upgrade can be triggered through the following methods:

* Through the web UI, by selecting an appropriate version on the "Updates" tab
* Executing the `gravity upgrade` CLI command on one of cluster nodes.
* Executing the `upgrade` script after unpacking the Application Bundle tarball
  on one of the cluster nodes.

#### Manual Upgrade Mode

In the manual mode, a user executes a sequence of commands on appropriate nodes of the
cluster according to a generated "Operation Plan". The upgrade operation in manual
mode is started by running the following command using the gravity binary:

```bash
$ ./gravity upgrade --manual
```

!!! tip
    Manual upgrade steps must be executed with the gravity binary included in the upgrade
    tarball to ensure version compatibility. If you don't have an installer tarball (for
    example, when downloading upgrades directly from connected Ops Center), you can obtain
    the appropriate gravity binary from the distribution Ops Center (see [Getting the Tools](quickstart.md#getting-the-tools)).

Once the upgrade operation has been initiated, the generated operation plan can be viewed
by running:

```bash
$ ./gravity plan
```

It will output the operation plan generated for the cluster that looks like this (it may differ
depending on what actually needs to be updated):

```bash
Phase               Description                                               State         Requires       Updated
-----               -----------                                               -----         --------       -------
* init             Initialize update operation                               Unstarted     -              -
* bootstrap        Bootstrap update operation on nodes                       Unstarted     /init          -
  * node-1         Bootstrap node "node-1"                                   Unstarted     /init          -
* masters          Update master nodes                                       Unstarted     /bootstrap     -
  * node-1         Update system software on master node "node-1"            Unstarted     /bootstrap     -
* runtime          Update application runtime                                Unstarted     /masters       -
  * rbac-app       Update system application "rbac-app" to 4.21.55           Unstarted     /masters       -
  * site           Update system application "site" to 4.21.55               Unstarted     /masters       -
  * kubernetes     Update system application "kubernetes" to 4.21.55-157     Unstarted     /masters       -
* app              Update installed application                              Unstarted     /masters       -
  * example        Update application "example" to 2.0.0                     Unstarted     /masters       -
```

The operation plan consists of multiple phases all of which need to be executed in order to complete
the operation. To execute a particular plan phase, run:

```bash
$ sudo ./gravity upgrade --phase=/init              # execute phase "init"
$ sudo ./gravity upgrade --phase=/bootstrap/node-1  # execute subphase "node-1" of the "bootstrap" phase, must be executed on node "node-1"
$ sudo ./gravity upgrade --phase=/runtime           # execute all subphases of the "runtime" phase
```

A couple of things to keep in mind:

* Some of the phases depend on other phases (indicated in the plan's "Requires" column) and
  may be executed only after the phases they depend on have been completed.
* Some of the phases (e.g. bootstrap or updating system software) have to be executed on a
  specific node, normally indicated by the name of its corresponding sub-phase, e.g. "node-1".

Invoke `gravity plan` to see which phases have been completed and which ones still need to
be executed. When all of the plan's phases have been successfully completed, finish the upgrade:

```bash
$ sudo ./gravity upgrade --complete
```

This will complete the operation and return the cluster back into active state.

#### Resuming

The update can be resumed with the `--resume` flag. This will resume the operation from the
last failed step. If a step has been marked as in-progress, a `--force` flag might be needed to
resume operation:

```bash
$ sudo ./gravity upgrade --resume --force
```

#### Rolling Back

In case something goes wrong during the upgrade, any phase can be rolled back by running:

```bash
$ sudo ./gravity rollback --phase=/masters/node-1
```

Failed/rolled back phases can be retried again. To abort the whole upgrade operation, rollback
all phases that have been completed and run `gravity upgrade --complete` command. It will
mark the operation as failed and move the cluster into active state.

### Troubleshooting Automatic Upgrades

!!! tip "Advanced Usage"
    This section covers the "under the hood" details of the automatic updates.

When a user initiates an automatic update by executing `gravity update`
command, the following actions take place:

1. An agent (update agent) is deployed and started on all cluster nodes. The
   update agents start on every cluster node as a `systemd` unit called
   `gravity-agent.service`.
2. The agents sequentially execute all phases of the update plan. These are
   the same phases a user would run as part of a [manual upgrade](#manual-upgrade-mode).
3. A successful update must be marked as "completed". This creates a checkpoint
   which a cluster can be rolled back to in case of future update failures.
4. Update agents are stopped.

Below is the list of the low level sub-commands executed by `gravity update`
to do all of this. These commands can be executed manually
from a machine in a Telekube Cluster:

```bash
# copy the update agent to every cluster node and start the agents:
$ ./gravity agent deploy

# request all nodnes to start sequentially executing the phases of the upgrade plan:
$ ./gravity upgrade --phase=<phase>

# complete an upgrade operation (to a state which you can rollback to):
$ ./gravity upgrade --complete

# shut down the update agents on all nodes:
$ ./gravity agent shutdown
```

If one of the update agents fails, an error message will be logged into the syslog
but the remaining agents on other cluster nodes will continue running. The status
of the update agent can be found by executing the following command on a failed node:

```bash
$ ./gravity agent shutdown
```

In case an automatic upgrade was interrupted, it can be resumed by executing:

```bash
$ ./gravity agent run --upgrade
```

### Separation of Workloads

Clusters with complex software deployed on them require separation of workloads between the control plane and application components to enable a seamless upgrade experience.

Telekube release 4.23+ leverages the support for node taints and tolerations in Kubernetes. Taints are special labels that control which resources can be scheduled onto a node. A pod needs to tolerate a taint in order to be schedulable on that particular node.

A system run-level controls load on a node required for operations like cluster update. In normal operation, application resources are scheduled according to the specifications including node labels and node/pod (anti-)affinity. When the cluster is operating under special conditions (like updating), application resources might be subject to runtime restriction until the operation is complete.

Being able to control node load is critical during update, as an overloaded node (kubelet) cannot guarantee uninterruptible and expected cluster operation and can lead to stuck updates.

The system run-level is a node configuration that prevents all but system components from being scheduled onto that node. The run-level is implemented as a node taint using the following key/value pair:

```
  gravitational.io/level=system
```

The update will perform as follows:

 1. Run the pre-update application hook to scale down resources for the duration of the update to further reduce the cluster load.
 1. Apply a system taint to the node so that only system resources are scheduled (implemented as a sub-phase of the masters phase: `/masters/node/taint`).
 1. Drain the node to be updated (implemented as a sub-phase of masters phase: `/masters/node/drain`).
 1. Update system software on the node.
 1. Mark the node schedulable to let the system applications run (implemented as a sub-phase of masters phase: `/masters/node/uncordon`).
 1. Remove the system taint after the system applications have started (implemented as a sub-phase of masters phase: `/masters/node/untaint`).
 1. After all nodes have gone through these steps, update the runtime (system applications).
 1. Perform the application update.
 1. Run post-upgrade hooks to scale the application back up.

Below is further detail on each step.

#### Run the pre-update application hook that scale down resources.

In order to scale down resources prior to the upgrade operation, the Application Manifest needs to be changed to accommodate a new hook:

```yaml
  preUpdate:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: application-pre-update
        namespace: default
      spec:
        activeDeadlineSeconds: 2400
        template:
          metadata:
            name: application-pre-update
          spec:
            restartPolicy: Never
            containers:
              - name: update
                image: application-hooks:latest
                command: ["/scale-down.sh"]
```

With the new `preUpdate` hook, the Telekube Cluster scales down the application resources in preparation for the update. The scaling logic needs to be aware of the cluster size to make appropriate scaling decisions.
The hook execution is implemented as a separate phase and is executed automatically in automatic upgrade mode.

You can find out more about other hook types on the [Telekube documentation on Packaging and Deployment](pack.md#application-hooks).

#### Apply a system taint to the node

The node taint is required so that the node will only schedule the system applications after a restart. The user application pods will be scheduled once the taint has been removed (see below for the step that does this).

```bash
$ sudo gravity upgrade --phase=/masters/<node-name>/taint
```

#### Drain the node to be updated

The node is drained before the system software on it is updated. Draining entails marking the node as unschedulable and evicting the running Pods (subject to Eviction API and PodDisruptionBudgets). The nodes are drained one by one to avoid disrupting too many resources.

This process serves multiple purposes:

- It helps the node to start in a clean state.
- It marks the resources as not running so that the scheduler and other controllers can make educated decisions.

```bash
$ sudo gravity upgrade --phase=/masters/<node-name>/drain
```

#### Update system software on the node

This step entails updating the system software on the node.

```bash
$ sudo gravity upgrade --phase=/masters/<node-name>
```

#### Make the node schedulable to start system applications

This is the reverse of the drain step. It removes the scheduling restriction and allows the scheduler to schedule pods on this node. However, the pods will only be scheduled if they tolerate the system taint.

This step is implemented as an additional phase:

```bash
$ sudo gravity upgrade --phase=/masters/<node-name>/uncordon
```
#### Remove the system taint

This step removes the artificial restriction imposed by the taint step (see above) and allows the application pods to be scheduled on this node.

```bash
$ sudo gravity upgrade --phase=/masters/<node-name>/untaint
```

#### Update the runtime

This step updates the runtime (system applications) and precedes the user application update.

```bash
$ sudo gravity upgrade --phase=/runtime
```

#### Perform the application update

This step executes the user application update hooks, after which the application is considered updated.

```bash
$ sudo gravity upgrade --phase=/app
```


#### Run post-upgrade hook to scale the application back up

The `postUpdate` hook is an opportunity to scale the application resources to their full capacity.
Just as with the `preUpdate` hook, the `postUpdate` hook needs to be added to the application manifest.

```yaml
  postUpdate:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: application-post-update
        namespace: default
      spec:
        activeDeadlineSeconds: 2400
        template:
          metadata:
            name: application-post-update
          spec:
            restartPolicy: Never
            containers:
              - name: update
                image: application-hooks:latest
                command: ["/scale-up.sh"]
```

The hook runs as part of the `/app` phase.

## Adding a Node

The `gravity` binary must be present on a node in order to add it to
the Telekube Cluster.

The node can then be added with the `gravity join` command:

```
$ sudo gravity join <peer-addr> --advertise-addr=<...> --token=<...>
```

Peer address is the address of one of the existing nodes of the cluster. The command accepts the following arguments:

Flag | Description
-----|------------
`--advertise-addr` | IP address the new node should be visible as.
`--token` | Token to authorize this node to join the cluster. Can be discovered by running `gravity status`.
`--role` | _(Optional)_ Role of the joining node. Autodetected if not specified.
`--state-dir` | _(Optional)_ Directory where all Telekube system data will be kept on this node. Defaults to `/var/lib/gravity`.

Every node in a Cluster must have a role, defined in the Application
Manifest. A role defines the system requirements for the node. For example, nodes
of "database" role must have storage attached to them. Telekube enforces the
system requirements for the role when adding a new node.

## Removing a Node

A node can be removed by using the `gravity leave` or `gravity remove`
commands.

To decommission an active node from the Cluster while on a particular node
run `gravity leave`.

If successful, the `leave` command will output the ID of the
initiated removal operation. The operation progress can be monitored
using the `gravity status` command.

During the decommissioning, all application and Kubernetes services running
on that node will be shut down, all Gravitational software and data removed and,
if using a cloud provisioner, the node itself will be deprovisioned. If the node
can access the rest of the Cluster, it will also be removed from the Cluster records.

In the event of the node being in an invalid state (for example, due to a failed
install or join), you can force the decommissioning:

```bash
$ gravity leave --force
```

A node can also be removed from the Cluster records by running `gravit remove` on any
node in the Cluster.

```bash
$ gravity remove <node>
```

If the node being removed is active in the Cluster, it will also be decommissioned locally.

`<node>` specifies the node to remove and can be either the node's assigned hostname
or its IP address (the one that was used as a "advertise address" or "peer address" during
install/join) or its Kubernetes name (can be obtained via `kubectl get nodes`).

## Recovering a Node

Let's assume you have lost the node with IP `1.2.3.4` and it can not be recovered.
Here are the steps to remove the lost node from the cluster and add a new node `1.2.3.7` to replace the faulty
one:

#### Remove the node from the system

On any node that has a functioning cluster running, execute the command `remove` to
forcefully remove the unavailable node from the cluster.

This command will update Etcd, Kubernetes and Telekube state to remove faulty node from the
database. Let's assume the remaining nodes are:

```bash
1.2.3.5
1.2.3.6
```

We are going to add `1.2.3.7` to replace the faulty `1.2.3.4`.

On the node `1.2.3.5`, execute this command:

```bash
gravity remove 1.2.3.4 --force
```

Wait until the operation completes by polling the status:

```bash
gravity status
Cluster:	adoringyalow5948, created at Tue May  2 16:21 UTC (2 hours ago)
    node-2 (1.2.3.5), Tue May  2 16:26 UTC
    node-3 (1.2.3.6), Tue May  2 16:29 UTC
Application:		telekube, version 3.35.5
Status:			active
Join token:		<join token>
Periodic updates:	OFF
```

Remember the join token. We will use it to add a new node as the next step:

#### Add new member to the cluster

Once the remove operation finishes, you can add the new node to the cluster.
Execute this command on node `1.2.3.7`.

```bash
sudo gravity join 1.2.3.5 --advertise-addr=1.2.3.7 --token=<join token>
```

Make sure that operation is completed by executing `gravity status`:

```bash
[vagrant@node-3 ~]$ gravity status
Cluster:	adoringyalow5948, created at Tue May  2 16:21 UTC (2 hours ago)
    node-2 (1.2.3.5), Tue May  2 16:26 UTC
    node-3 (1.2.3.6), Tue May  2 16:29 UTC
    node-4 (1.2.3.7), Tue May  2 18:30 UTC
Application:		telekube, version 3.35.5
Status:			active
Join token:		<join token>
Periodic updates:	OFF
```

You should see the third node registered in the cluster and cluster status set to `active`.

#### Autoscaling the cluster

When running on AWS, Telekube integrates with [Systems manager parameter store](http://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-paramstore.html) to simplify the discovery.

Assuming the cluster name is known, a new node can join the cluster using `gravity autojoin` command

```bash
sudo gravity autojoin example.com --role=knode
```

`gravity discover` will locate the cluster load balancer and join token by reading parameter store
and automatically connect. This command can be run as a part of `cloud-init` of the AWS instance.

Users can read more about AWS integration [here](https://github.com/gravitational/provisioner#provisioner)

## Backup And Restore

Telekube Clusters support backing up and restoring the application state. To enable backup
and restore capabilities, the application should define "backup" and "restore" hooks in the
manifest. These hooks are Kubernetes jobs that have `/var/lib/gravity/backup` directory
mounted into them. The hooks are run on the same node the backup/restore that the command is invoked
upon.

The backup hook collects all data it wishes to back up into this directory and the restore
hook reads the backed up data from it. Here is an example of backup/restore
hooks for the application secrets:

```yaml
hooks:
  backup:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: backup
      spec:
        template:
          metadata:
            name: backup
          spec:
            restartPolicy: OnFailure
            containers:
              - name: hook
                image: quay.io/gravitational/debian-tall:0.0.1
                command:
                  - "/bin/sh"
                  - "-c"
                  - "/usr/local/bin/kubectl get secrets --namespace=default -o=yaml > /var/lib/gravity/backup/backup.yaml"
  restore:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: restore
      spec:
        template:
          metadata:
            name: restore
          spec:
            restartPolicy: OnFailure
            containers:
              - name: hook
                image: quay.io/gravitational/debian-tall:0.0.1
                command:
                  - "/usr/local/bin/kubectl"
                  - "apply"
                  - "-f"
                  - "/var/lib/gravity/backup/backup.yaml"
                  - "--namespace"
                  - "default"
```

To trigger a backup, log into one of the cluster nodes and execute:

```bash
root$ gravity backup <data.tar.gz>
```

where `<data.tar.gz>` is the name of the output backup archive. The tarball contains
compressed contents of `/var/lib/gravity/backup` directory from the backup hook.

To restore the data from the backup tarball:

```bash
root$ gravity restore <data.tar.gz>
```

!!! tip
    You can use `--follow` flag for backup/restore commands to stream hook logs to
    standard output.

## Remote Assistance

Every Telekube cluster can be connected to an Ops Center,
assuming the cluster is connected to the Internet. This creates an outbound SSH tunnel from the cluster
to the Ops Center and the operator can use that tunnel to perform remote troubleshooting
by using the `tsh` tool. You can read more about remote assistance in the
[remote management](manage.md) section.

However, some Telekube Cluster owners may want to disable the SSH tunnel and keep their
clusters disconnected from the vendor, only enabling this capability when they
need help.

Remote Assistance can be turned on or off in the web UI or by using the
`gravity tunnel` command:

```bash
$ gravity tunnel <on|off>
```

When executed without an argument, `gravity tunnel` prints the current status
of the tunnel. The status can be one of the following

| Status | Description
|-------|--------------------------------------------------------------------------------------------------------------------------------------------------------|
| on    | The tunnel is turned on and the connection to the Ops Center is established.                                                                            |
| off   | The tunnel is turned off.                                                                                                                              |
| error | The tunnel is turned on, but the connection cannot be established due to an error. The additional error information will be printed as well. |

## Deleting a Cluster

### Bare Metal Cluster

To delete a cluster installed on existing machines you can execute `gravity leave` on
them to gradually shrink the cluster to the size of a single node (see [Removing a Node](#removing-a-node)
section for more information). After that, run `gravity leave --force` on the remaining
node to uninstall the application as well as all Telekube software and data.

### Cloud Provider Cluster

When installing a cluster using a cloud provider (e.g. AWS), all provisioned resources
are marked with a tag `KubernetesCluster` equal to the domain name that was chosen
during the installation. To deprovision an AWS cluster, log into your AWS control panel
and delete all of the resources marked with the appropriate `KubernetesCluster` tag in
the following order:

* Instances
* Load Balancers (it deletes in the background so might take a while before it's fully gone)
* Security Groups
* VPC (will delete all associated resources like Internet Gateway, Subnet & Route Table as well)

!!! tip "Resource Groups"
    On AWS you can use `Resource Groups` to get a unified view of all resources matching
    a particular criteria. Create a resource group filtering by an appropriate
    `KubernetesCluster` tag so see all AWS resources for a cluster.

## Troubleshooting

To collect diagnostic information about a cluster (e.g. to submit a bug report or get assistance in troubleshooting cluster problems),
use the `report` command:

```bash
$ gravity report --help

usage: gravity report [<flags>]

get cluster diagnostics report

Flags:
  --help       Show help (also see --help-long and --help-man).
  --debug      Enable debug mode
  -q, --quiet  Suppress any extra output to stdout
  --insecure   Skip TLS verification
  --log-file="/var/log/telekube-install.log"
               log file with diagnostic information
  --file="report.tar.gz"
               target report file name

Example:

$ gravity report
```

This command will collect diagnostics from all cluster nodes into the specified tarball that you can then
submit to engineering for evaluation.

## Configuring a Cluster

Telekube borrows the concept of resources from Kubernetes to configure itself.
Use `gravity resource` command to update the cluster configuration.

Currently supported resources are:

Resource Name             | Resource Description
--------------------------|---------------------
`oidc`                    | OIDC connector
`role`                    | cluster role
`user`                    | cluster user
`token`                   | user tokens such as API keys
`logforwarder`            | forwarding logs to a remote rsyslog server
`trusted_cluster`         | managing access to remote Ops Centers
`endpoints`               | Ops Center endpoints for user and cluster traffic
`cluster_auth_preference` | cluster authentication settings such as second-factor

### Configuring OpenID Connect

An Telekube Cluster can be configured to authenticate users using an
OpenID Connect (OIDC) provider such as Auth0, Okta and others.

A resource file in YAML format creates the connector.  Below is an
example of an OIDC resource for provider "Auth0" called
`oidc.yaml`:

```yaml
kind: oidc
version: v2
metadata:
  name: auth0
spec:
  redirect_url: "https://telekube-url/portalapi/v1/oidc/callback"
  client_id: <client id>
  client_secret: <client secret>
  issuer_url: "https://example.com/"
  scope: [roles]
  claims_to_roles:
    - {claim: "roles", value: "gravitational/admins", roles: ["@teleadmin"]}
```

Add this connector to the cluster:

```bash
$ gravity resource create oidc.yaml
```

To list the installed connectors:

```bash
$ gravity resource get oidc
```

To remove the connector `auth0`:

```bash
$ gravity resource rm oidc auth0
```

#### Google OIDC Connector Example

Here's an example of the OIDC connector that uses Google for authentication:

```yaml
kind: oidc
version: v2
metadata:
  name: google
spec:
  redirect_url: "https://ops-advertise-url/portalapi/v1/oidc/callback"
  client_id: <client id>
  client_secret: <client secret>
  issuer_url: https://accounts.google.com
  scope: [email]
  claims_to_roles:
    - {claim: "hd", value: "example.com", roles: ["@teleadmin"]}
```

The `hd` scope contains the hosted Google suite domain of the user so in the
above example, any user who belongs to the "example.com" domain will be
allowed to log in and granted the admin role.

!!! note 
    The user must belong to a hosted domain, otherwise the `hd` claim will
    not be populated.

### Configuring Roles

Below is an example of a resource file with the definition of an admin role. The admin has
access to all resources, including roles, other users and authentication settings such
as OIDC connectors, and belongs to a privileged Kubernetes group:

```yaml
kind: role
version: v2
metadata:
  name: administrator
spec:
  resources:
    "*":
      - read
      - write
  clusters:
    - "*"
  generate_licenses: true
  kubernetes_groups:
    - admin
  logins:
    - root
  max_session_ttl: "30h0m0s"
  namespaces:
    - "*"
  repositories:
    - "*"
```

Below is an example of a non-admin role spec providing access to a particular
cluster `example.com` and its applications:

```yaml
kind: role
version: v2
metadata:
  name: developer
spec:
  resources:
    cluster:
      - read
      - write
    app:
      - read
      - write
  clusters:
    - example.com
  kubernetes_groups:
    - admin
  logins:
    - root
  max_session_ttl: "10h0m0s"
  namespaces:
    - default
  repositories:
    - "*"
```

To create these two roles you can execute:

```bash
$ gravity resource create administrator.yaml
$ gravity resource create developer.yaml
```

To view all currently available roles:

```bash
$ gravity resource get role
```

To delete the `developer` role:

```bash
$ gravity resource delete role developer
```

### Configuring Users & Tokens

Below is an example of a resource file that creates a user called `user.yaml`.

```yaml
kind: user
version: v2
metadata:
  name: "alice@example.com"
spec:
  # "agent" type means this user is only authorized to access the cluster
  # using the API key (token) and not using the web UI
  type: agent
  roles: ["developer"]
```

Create the user by executing `gravity resource`:

```bash
$ gravity resource create user.yaml
```

A token can is assigned to this user by using
the following resource file called `token.yaml`:

```yaml
kind: token
version: v2
metadata:
   name: xxxyyyzzz
spec:
   user: "alice@example.com"
```

Create the token by executing `gravity resource`:

```bash
$ gravity resource create token.yaml
```

To view available users and a user's tokens:

```bash
$ gravity resource get user
$ gravity resource get token --user=alice@example.com
```

### Example: Provisioning A Publisher User

In this example we are going to use `role`, `user` and `token` resources described above to
provision a user who can publish applications into an Ops Center. For instructions on how
to setup your own Ops Center see [Setting up an Ops Center](opscenter.md).

In the following spec we define 3 resources:

1. role `publisher` that allows its holder to push applications
2. user `jenkins@example.com` that carries this role
3. token that will be used for authenticating with Ops Center

```yaml
kind: role
version: v3
metadata:
  name: publisher
spec:
  allow:
    logins:
    - publisher
    namespaces:
    - default
    rules:
      - resources: [app]
        verbs: [read, create, update, delete, list]
---
kind: user
version: v2
metadata:
  name: "jenkins@example.com"
spec:
  type: "agent"
  roles: ["publisher"]
---
kind: token
version: v2
metadata:
   name: "s3cr3t!"
spec:
   user: "jenkins@example.com"
```

Save the resources into `publisher.yaml` and create them on the cluster:

```bash
$ gravity resource create publisher.yaml
```

This is how the new user can publish new application bundles into the Ops Center:

```bash
$ tele login -o opscenter.example.com --token=s3cr3t!
$ tele build yourapp.yaml -o installer.tar
$ tele push installer.tar
```

### Example: Provisioning A Cluster Admin User

The example below shows how to create an admin user for a cluster.
Save the user definition into a YAML file:

```yaml
# admin.yaml
kind: user
version: v2
metadata:
  name: "admin@example.com"
spec:
  type: "admin"
  password: "Passw0rd!"
  roles: ["@teleadmin"]
```

The password will be encrypted with
[bcrypt](https://en.wikipedia.org/wiki/Bcrypt) prior to being saved into the
database. Note the role `@teleadmin` - this is a built-in system role for the
cluster administrators.

To create the user from the YAML above, execute the following command on one of
the cluster nodes:

```bash
$ gravity resource create admin.yaml
```

The new user can now log into the cluster via the Web UI with the user
credentials created above.

!!! tip "Password Restrictions"
    Passwords must be between 6 and 128 characters long.

### Configuring Log Forwarders

Every Telekube Cluster is automatically set up to aggregate the logs from all
running containers. By default the logs are kept inside the Cluster but they can be configured to be
shipped to a remote log collector such as a rsyslog server.

Below is a sample resource file called `forwarder.yaml` that creates a log forward:

```yaml
kind: logforwarder
version: v2
metadata:
   name: forwarder1
spec:
   address: 192.168.100.1:514
   protocol: udp
```

The `protocol` field is optional and defaults to `tcp`. Create the log forwarder:

```bash
$ gravity resource create forwarder.yaml
```

To view currently configured log forwarders, run:

```bash
$ gravity resource get logforwarders
```

To delete a log forwarder:

```bash
$ gravity resource rm logforwarder forwarder1
```

### Configuring TLS Key Pair

Ops Center and Telekube Cluster Web UI and API TLS key pair can be configured
using `tlskeypair` resource.

```yaml
kind: tlskeypair
version: v2
metadata:
  name: keypair
spec:
  private_key: |
    -----BEGIN RSA PRIVATE KEY-----
  cert: |
    -----BEGIN CERTIFICATE-----
```

!!! tip "Certificate chain"
    `cert` section should include all intermediate certificate PEM blocks concatenated to function properly!

To update the key pair:

```bash
$ gravity resource create tlskeypair.yaml
```

To view currently configured key pair:

```bash
$ gravity resource get tls
```

To delete a TLS key pair (in this case default self-signed TLS key pair will be used instead):

```bash
$ gravity resource rm tls keypair
```

### Configuring Trusted Clusters

!!! note
    Support for trusted clusters is available since Telekube version
    `5.0.0-alpha.5`.

Trusted clusters is a concept for connecting standalone Telekube clusters to
arbitrary Ops Centers. It brings the following advantages:

* Allows agents of the remote Ops Center to SSH into your cluster nodes to
perform remote assistance.
* Allows the cluster to download application updates from the Ops Center.

To configure a trusted cluster create the following resource:

```yaml
kind: trusted_cluster
version: v2
metadata:
  name: opscenter.example.com
spec:
  enabled: true
  pull_updates: true
  token: c523fd0961be71a45ceed81bdfb61b859da8963e2d9d7befb474e47d6040dbb5
  tunnel_addr: "opscenter.example.com:3024"
  web_proxy_addr: "opscenter.example.com:32009"
```

Let's go over the resource fields:

* `metadata.name`: The name of the Ops Center the cluster is being connected to.
* `spec.enabled`: Allows the agents to establish remote connection to the cluster
from the Ops Center.
* `spec.pull_updates`: Whether the cluster should be automatically downloading
application updates from the Ops Center.
* `spec.token`: A secret token used to securely connect the cluster to the Ops
Center. Can be retrieved by running `gravity status` command on the Ops Center
cluster.
* `spec.tunnel_addr`: The address of the Ops Center reverse tunnel service as
host:port. Typically it is exposed on port `3024`.
* `spec.web_proxy_addr`: The address which the Ops Center cluster serves its web
API on. It is the address specified via the `--ops-advertise-addr` parameter
in the [Manual Provisioning](opscenter.md#manual-provisioning) flow (the first
port).

Create the trusted cluster:

```bash
$ gravity resource create trustedcluster.yaml
```

View the currently configured trusted cluster:

```bash
$ gravity resource get trustedclusters
Name                       Enabled     Pull Updates
----                       -------     ------------
opscenter.example.com      true        true
```

Once the cluster has been created, the reverse tunnel status can be viewed and
managed using `gravity tunnel` shortcut commands:

```bash
$ gravity tunnel status
Ops Center              Status
opscenter.example.com   enabled

$ gravity tunnel disable
Ops Center              Status
opscenter.example.com   disabled

$ gravity tunnel enable
Ops Center              Status
opscenter.example.com   enabled
```

To disconnect the cluster from the Ops Center, remove the trusted cluster:

```bash
$ gravity resource rm trustedcluster opscenter.example.com
trusted cluster "opscenter.example.com" has been deleted
```

### Configuring Ops Center Endpoints

By default an Ops Center is configured with a single endpoint set
via `--ops-advertise-addr` flag during the installation. This means that
all Ops Center clients (such as users of the Ops Center UI or tele/tsh tools
as well as remote clusters) will use this address to connect to it.

Ops Center can be configured to advertise different addresses to users and
remote clusters via the `endpoints` resource. It has the following format:

```yaml
kind: endpoints
version: v2
metadata:
  name: endpoints
spec:
  public_advertise_addr: "<public-host>:<public-port>"
  agents_advertise_addr: "<agents-host>:<agents-port>"
```

* `public_advertise_addr` is the address that will be used for Ops Center
  UI and by CLI tools such as tele or tsh. This field is mandatory.
* `agents_advertise_addr` is the address that remote clusters will use to
  connect to Ops Center. This field is optional and it falls back to the
  public address if not specified.

Create the resource to update the Ops Center endpoints:

```bash
$ gravity resource create endpoints.yaml
```

!!! note 
    Updating the endpoints resource will result in restart of `gravity-site`
    pods so the changes can take effect.

To view currently configured endpoints, run:

```bash
$ gravity resource get endpoints
```

Let's take a look at how Ops Center behavior changes with different endpoint
configurations.

#### Single advertise address

This is the default configuration, when `agents_advertise_addr` is either
not specified or equals to `public_advertise_addr`:

```yaml
spec:
  public_advertise_addr: "ops.example.com:443"
```

With this configuration, the Ops Center cluster will provide a single Kubernetes
service called `gravity-public` configured to serve both user and cluster
traffic:

```bash
$ kubectl get services -n kube-system -l app=gravity-opscenter
NAME             TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)                                       AGE
gravity-public   LoadBalancer   10.100.20.71   <pending>     443:31033/TCP,3024:30561/TCP,3023:31043/TCP   40m
```

!!! tip "Setting up ingress"
    On cloud installations that support Kubernetes integration such as AWS, a
    load balancer will be created automatically, so you will only need to
    configure DNS to point the advertised hostname (`ops.example.com` in this
    example) to it. For onprem installations, an ingress should be configured
    for the appropriate NodePort of the service (`31033` in this example).

#### Same hostname, different port

In this scenario both user and cluster traffic should be accessible on
the same hostname but on different ports:

```yaml
spec:
  public_advertise_addr: "ops.example.com:443"
  agents_advertise_addr: "ops.example.com:4443"
```

With this configuration, the Ops Center will provide a single Kubernetes service
called `gravity-public` (which `ops.example.com` can point at) with two
different ports for user and cluster traffic respectively:

```bash
kubectl get services -n kube-system -l app=gravity-opscenter
NAME             TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)                                                      AGE
gravity-public   LoadBalancer   10.100.20.71   <pending>     443:31265/TCP,4443:30080/TCP,3024:32109/TCP,3023:30716/TCP   54m
```

#### Different hostnames

In this scenario user and cluster traffic have different advertise hostnames:

```yaml
spec:
  public_advertise_addr: "ops.example.com:443"
  agents_advertise_addr: "ops-agents.example.com:4443"
```

The ports may be the same or different which does not affect the general
behavior, only the respective service configuration.

With this configuration, an additional Kubernetes service called `gravity-agents`
is created for the cluster traffic which `ops-agents.example.com` can be point at:

```bash
# kubectl get services -n kube-system -l app=gravity-opscenter
NAME             TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)                         AGE
gravity-public   LoadBalancer   10.100.20.71    <pending>     443:31792/TCP,3023:32083/TCP    59m
gravity-agents   LoadBalancer   10.100.91.204   <pending>     4443:30873/TCP,3024:30185/TCP   8s
```

### Configuring Cluster Authentication Preference

Cluster authentication preference resource allows to configure method of
authentication users will use when logging into a Telekube cluster.

The resource has the following format:

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: auth-oidc
spec:
  # preferred auth type, can be "local" (to authenticate against
  # local users database) or "oidc"
  type: oidc
  # second-factor auth type, can be "off" or "otp"
  second_factor: otp
```

By default the following authentication method is configured:

* For Ops Centers: OIDC or local with second-factor authentication.
* For regular clusters: local without second-factor authentication.

To update authentication preference, for example to allow local users to log
into an Ops Center without second-factor, define the following resource:

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: auth-local
spec:
  type: local
  second_factor: "off"
```

Create it:

```bash
$ gravity resource create auth.yaml
```

!!! note 
    Make sure to configure a proper [OIDC connector](cluster.md#configuring-openid-connect)
    when using "oidc" authentication type.

To view the currently configured authentication preference:

```bash
$ gravity resource get cluster_auth_preference
Type      ConnectorName     SecondFactor
----      -------------     ------------
local                       off
```

!!! note 
    Currently authentication preference only affects login via web UI,
    `tele login` will add support for it in the future.

## Managing Users

Telekube cluster allows to invite new users and reset passwords for existing
users by executing CLI commands on the cluster nodes.

### Invite User

To invite a user execute the `gravity users add <username>` command on one of
the cluster nodes. The command accepts the following flags:

Flag      | Description
----------|-------------
`--roles` | List of roles to assign to a new user. The built-in role `@teleadmin` grants admin permissions.
`--ttl`   | Time to live (TTL) for the invite token. Examples: "5h", "10m", "1h30m", The default is "8h", maximum is "48h".

The command will generate a signup URL valid for the specified amount of time:

```bash
$ gravity users add alice@example.com --roles=@teleadmin --ttl=1h
Signup token has been created and is valid for 1h0m0s hours. Share this URL with the user:
https://<host>/web/newuser/<token>
```

!!! note 
    Make sure that `<host>` is accessible to the invited user.

### Reset User Password

To reset a password for an existing user execute the `gravity users reset <username>`
command on one of the cluster nodes. The command accepts the following flags:

Flag      | Description
----------|-------------
`--ttl`   | Time to live (TTL) for the reset token. Examples: "5h", "10m", "1h30m", The default is "8h", maximum is "24h".

The command will generate a password reset URL valid for the specified amount of
time:

```bash
$ gravity users reset alice@example.com --ttl=1h
Password reset token has been created and is valid for 1h0m0s. Share this URL with the user:
https://<host>/web/reset/<token>
```

!!! note 
    Make sure that `<host>` is accessible to the user.

## Securing a Cluster

Telekube comes with a set of roles and bindings (for role-based access control or RBAC) and a set of pod security policies. This lays the ground for further security configurations.

### Pod security policies

Introduced in Kubernetes 1.5, Pod security policies allow controlled access to privileged containers based on user roles and groups. A Pod security policy specifies what a Pod can do and what it has access to.
Policies allow the administrator to control many facets of the system (See [PodSecurityPolicies] for details).

By default Telekube provides two security policies: `privileged` and `restricted`.

  * A `restricted` policy has the following attributes:
     1. limits the range of user IDs Pods can run as
     2. disallows certain capabilities (`NET_ADMIN`, `SYS_ADMIN`, `SYS_CHROOT`)
     3. disables host networking

  * A `privileged` policy is a permissive policy allowing unrestricted access within cluster.

Two `ClusterRoles` - `restricted-psp-user` and `privileged-psp-user` enable access
to the respective security policies.
Additionally, the two default groups - `restricted-psp-users` and `privileged-psp-users`
can be used to assign privileges to users and other groups:

  * `restricted-psp-users` group grants access to the restricted role
  * `privileged-psp-users` group grants access to both roles

Finer control can be exercised by modifying the existing or creating new security policies,
cluster roles and role bindings.

For [example](https://github.com/kubernetes/examples/blob/master/staging/podsecuritypolicy/rbac/policies.yaml), to restrict the types of volumes Pods can use (among other things), create the following policy `psp-volumes.yaml`:

```yaml
apiVersion: extensions/v1beta1
kind: PodSecurityPolicy
metadata:
  name: psp-volumes
spec:
  privileged: false
  fsGroup:
    rule: RunAsAny
  runAsUser:
    rule: MustRunAsNonRoot
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - 'emptyDir'
  - 'secret'
  - 'downwardAPI'
  - 'configMap'
  - 'persistentVolumeClaim'
```

```bash
$ kubectl apply -f psp-volumes.yaml
```

Create a role to enable the use of the policy `psp-roles.yaml`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: psp-volumes-user
rules:
- apiGroups:
  - extensions
  resources:
  - podsecuritypolicies
  resourceNames:
  - psp-volumes
  verbs:
  - use
```

```bash
$ kubectl apply -f psp-roles.yaml
```

Create a `ClusterRoleBinding` to be able to assign users to the new `psp-volume-users` group `psp-bindings.yaml`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: psp-volume-users
subjects:
- kind: Group
  name: psp-volume-users
- kind: ServiceAccount
  name: psp-volume-account
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: psp-volume-user
```

Resources are then created using the new policy by either using a certificate with
the new group or the new `psp-volume-account` service account.

### RBAC

By default Telekube:

  * connects default service accounts (in `default` and `kube-system` namespaces) to the built-in `cluster-admin` role
  * creates `admin`/`edit`/`view` Telekube specific groups (bound to respective built-in cluster roles)

See the Kubernetes [RBAC] documentation for more information.


## Eviction Policies

Kubernetes includes support for evicting pods in order to maintain node stability, which is documented [here](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).

Telekube uses the following eviction policies by default:

 - Hard eviction
    1. less than 10% of disk space is available (`nodefs.available<10%`)
    1. less than 5% of the inodes are available (`nodefs.inodesFree<5%`)
    1. less than 15% of the image filesystem is free (`imagefs.available<15%`)
    1. less than 5% of the inodes on the image filesystem are available (`imagefs.inodesFree<5%`)

 - Soft eviction
    1. less than 20% of disk space is available (`nodefs.available<20%`)
    1. less than 10% of the inodes are available (`nodefs.inodesFree<10%`)
    1. less than 20% of the image filesystem is free (`imagefs.available<20%`)
    1. less than 10% of the inodes on the image filesystem are available (`imagefs.inodesFree<10%`)

The default grace period for soft eviction policy is set to 1 hour.

These defaults can be overridden by providing an alternative set of options for kubelet in the application manifest.

For example, to extend the default policy with an eviction threshold for available memory, you will need to specify both the defaults
and the new value:

```yaml
systemOptions:
  kubelet:
    args:
    - --eviction-hard="nodefs.available<10%,imagefs.available<15%,nodefs.inodesFree<5%,imagefs.inodesFree<5%,memory.available<1Gi"
```

## Custom Taints

Telekube supports provisioning nodes with [Taints](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) to control
workload distribution. Use `taints` property of the node profiles:

```yaml
nodeProfiles:
  - name: node
    description: "Telekube Node"
    taints:
    - key: custom-taint
      value: custom-value
      effect: NoExecute
```

### Dedicated control plane nodes

To install Kubernetes nodes running only system components, Telekube supports `node-role.kubernetes.io/master` taint:

```yaml
nodeProfiles:
  - name: master
    description: "Telekube Master"
    taints:
    - key: node-role.kubernetes.io/master
      effect: NoSchedule
```

## Node Roles

Users can assign cluster roles (e.g. Kubernetes master or node) using special labels `node-role.kubernetes.io/master` and
`node-role.kubernetes.io/node`.

If installer encounters a node labelled with `node-role.kubernetes.io/master: "true"` label, it will set up the node
as a Kubernetes master node running Etcd as a part of the cluster:

```yaml
nodeProfiles:
  - name: node
    description: "Telekube Master Node"
    labels:
      node-role.kubernetes.io/master: "true"
```

If the node is has been assigned the `node-role.kubernetes.io/node: "true"` label, it will be set up as a Kubernetes node,
i.e. will run Etcd configured in proxy mode:

```yaml
nodeProfiles:
  - name: node
    description: "Telekube Node"
    labels:
      node-role.kubernetes.io/node: "true"
```

If none of the labels above are set, Telekube will automatically assign node roles according to the following algorithm:

  * If there are already 3 master nodes available (either explicitly set via labels or already installed/elected
    in the system) - assign as kubernetes node.
  * Otherwise promote the node to a kubernetes master.


## Networking

### Hairpin NAT

For services that load-balanace to themselves, hairpin NAT needs to be configured through a dedicated configuration
parameter for kubelet in the application manifest:

```yaml
systemOptions:
  kubelet:
    hairpinMode: "hairpin-veth" # also, "promiscuous-bridge" or "none"
```

The `hairpinMode` attribute accepts the same values as kubelet.
See [kubelet's command line reference](https://kubernetes.io/docs/reference/generated/kubelet/) for details.

With `hairpin-veth`, kubelet/network plugin handles the setup.

With `promiscuous-bridge`, the behavior is similar to that of the kubenet network plugin:

  - the docker bridge is placed into promiscuous mode and
  - ebtables rules are set up to avoid duplicate packets as the bridge starts to
    accept traffic otherwise destined for other interfaces


!!! tip "Default hairpin mode"
    For 4.x, the default value for `systemOptions.kubelet.hairpinMode` is `hairpin-veth`.
    For 5.x, the default value for `systemOptions.kubelet.hairpinMode` is `promiscuous-bridge`.


!!! tip "Kernel module"
    In "promiscuous-bridge" mode, the nodes require a kernel module called `ebtable_filter` to manage ebtable rules,
    see [Kernel Modules](requirements.md#kernel-modules) for details.



[//]: # (Footnotes and references)

[PodSecurityPolicies]: https://kubernetes.io/docs/concepts/policy/pod-security-policy/
[RBAC]: https://kubernetes.io/docs/admin/authorization/rbac/
[promiscuous-mode]: https://en.wikipedia.org/wiki/Promiscuous_mode
