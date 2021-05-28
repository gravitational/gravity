---
title: Managing a Kubernetes Cluster with Gravity
description: How to manage the life cycle of an air-gapped or on-prem Kubernetes cluster with Gravity
---

# Cluster Management

This chapter covers Gravity Cluster administration.

Every application packaged with Gravity ("Application Bundle") is a self-contained Kubernetes system.
This means that every application running on a cluster ("Gravity Cluster" or "Cluster") consists
of the following components:

1. The application and its services: workers, databases, caches, etc.
2. Kubernetes services and CLI tools.
3. Gravity tooling such as the [Teleport SSH server](https://gravitational.com/teleport)
   and the Gravity CLI.

Gravity uses software we call "Gravity" for managing Clusters. The Gravity CLI is
the Gravity interface that can be used to manage the Cluster. Each Gravity Cluster
also has a graphical, web-based UI to view and manage the Cluster.

You can also use familiar Kubernetes tools such as `kubectl` to perform regular cluster
tasks such as watching logs, seeing stats for pods or volumes or managing configurations.

## Kubernetes Environment

Every Gravity Cluster is a standalone instance of Kubernetes running on
multiple nodes, so the standard Kubernetes tools will work and Kubernetes rules
will apply.

However, Gravity pre-configures Kubernetes to be as reliable as possible
greatly reducing the need for ongoing active management. To make this possible
on a node level, Gravity runs all Kubernetes services from a single executable
called `gravity`.

!!! tip "Gravity Master Container"
    When `gravity` process starts, it creates it's own container which we'll
    refer to as "master container". The master container itself is a
    containerized `systemd` instance. It launches the required Kubernetes
    daemons and contains their dependencies as well as monitoring and support
    tools.  This allows Kubernetes nodes to be as self-contained and immune to
    pre-existing node state as possible.

Running Kubernetes services from inside a master container brings several
advantages as opposed to installing Kubernetes components in a more traditional
way:

* Since all Kubernetes services such as `kubelet` or `kube-apiserver` always
  run enclosed inside the master container, it makes it possible for Gravity
  to closely monitor Kubernetes' health and perform cluster updates "from below".

* Gravity _continuously_ maintains Kubernetes configuration to be highly
  available ("HA").  This means that any node can go down without disrupting
  Kubernetes' operation.

* Gravity runs its own local Docker registry which is used as a cluster-level
  cache for container images. This makes application updates and restarts
  faster and more reliable.

* Gravity provides the ability to perform cluster state snapshots as part of
  cluster updates or to be used independently.

### Kubernetes Extensions

Gravitational also offers various extensions for Kubernetes that can be
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
3. Monitor Gravity Cluster health.
4. Update / backup / restore the Gravity Cluster.
5. Request shell inside the master container to troubleshoot Kubernetes health on a node level.


`gravity` commands can be used on any node of the cluster. Or you can execute it
remotely via an SSH tunnel by chaining it to the end of the `tsh ssh` command.

The full list of `gravity` commands:

| Command   | Description                                                        |
|-----------|--------------------------------------------------------------------|
| status    | show the status of the cluster and the application running in it   |
| update    | manage application updates on a Gravity Cluster                    |
| upgrade   | manage the cluster upgrade operation for a Gravity Cluster         |
| plan      | manage operation plan                                              |
| join      | add a new node to the cluster                                      |
| autojoin  | join the cluster using cloud provider for discovery                |
| leave     | decommission a node: execute on a node being decommissioned        |
| remove    | remove the specified node from the cluster                         |
| backup    | perform a backup of the application data in a cluster              |
| restore   | restore the application data from a backup                         |
| tunnel    | manage the SSH tunnel used for the remote assistance               |
| report    | collect cluster diagnostics into an archive                        |
| resource  | manage cluster resources                                           |
| exec      | execute commands in the master container                           |
| shell     | launch an interactive shell in the master container                |
| gc        | clean up unused cluster resources                                  |


## Cluster Status

Running `gravity status` will give you the high level overview of the cluster health,
as well as the status of its individual components.

Here's an example of how to remotely log in to a cluster via `tsh` and check the
status of the cluster named "production":

```bsh
$ tsh --cluster=production ssh admin@node gravity status
```

!!! tip "Reminder"
    Keep in mind that `tsh` always uses the Gravity Ops Center as an SSH proxy. This
    means the command above will work with clusters located behind
    corporate firewalls. You can read more in the [remote management](manage.md) section.

### Cluster Health Endpoint

Clusters expose an HTTP endpoint that provides system health information about
cluster nodes. Run the following command from one of the cluster nodes to query
the status:

```bsh
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

```bsh
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

### Cluster Status History

Running `gravity status history` displays the history of changes to the
Cluster status.

Example output may look something like the following:

```bsh
$ gravity status history
2020-02-18T01:36:07Z [Node Degraded]     node=node-1
2020-02-18T21:36:11Z [Node Degraded]     node=node-2
2020-02-18T21:36:25Z [Node Degraded]     node=node-3
2020-02-18T21:36:56Z [Probe Succeeded]   node=node-1 checker=node-status
2020-02-18T21:36:58Z [Probe Succeeded]   node=node-2 checker=node-status
2020-02-18T21:36:58Z [Probe Succeeded]   node=node-2 checker=time-drift
2020-02-18T21:36:59Z [Probe Succeeded]   node=node-3 checker=node-status
2020-02-18T21:37:07Z [Probe Succeeded]   node=node-1 checker=kube-apiserver
2020-02-18T21:37:07Z [Node Recovered]    node=node-1
2020-02-18T21:37:08Z [Probe Succeeded]   node=node-2 checker=kube-apiserver
2020-02-18T21:37:08Z [Node Recovered]    node=node-2
2020-02-18T21:37:11Z [Probe Succeeded]   node=node-3 checker=kube-apiserver
2020-02-18T21:37:11Z [Node Recovered]    node=node-3
```

Here's an example of how to view the history remotely via `tsh`:

```bsh
$ tsh --cluster=production ssh admin@node gravity status history
```

The `gravity status history` command is an additional tool to help debug issues
with a Cluster. The `gravity status` command only displays the current status
and provides limited visibility into the state of the Cluster. The
`gravity status history` command is there to help fill in the gaps. The history
lets you observe when and where problems have occurred within the Cluster.

There are just a few event types that are currently being tracked.
- `Node Degraded` / `Node Recovered` specifies a change in the node status. The node
key specifies the name of the node (node-1, node-2, node-3).
- `Probe Succeeded` / `Probe Failed` specifies a change in a probe result. The checker
key specifies the name of the health check (time-drift, kube-apiserver).

The `gravity status history` command is available on all `master` nodes of the
cluster and provides an eventually consistent history between nodes.

## Application Status

Gravity provides a way to automatically monitor the application health.

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

Any Gravity cluster can be explored using the standard Kubernetes tool, `kubectl`, which is
installed and configured on every cluster node. See the command's [overview](http://kubernetes.io/docs/user-guide/kubectl-overview/)
and a [full reference](https://kubernetes.io/docs/user-guide/kubectl/) to see what it can
do, or simply use `kubectl --help`.

Each Gravity Cluster also has a graphical UI to explore and manage the Cluster. To log into
the Cluster Admin UI you need to create an admin user. Please see the
[Custom Installer Screens](pack.md#custom-installation-screen) chapter for details on how
to enable a post-install screen that will let you create a local user.

## Updating a Cluster

Cluster updates can get quite complicated for complex cloud applications
composed of multiple micro-services. On a high level, there are two major layers
that will need periodic updating:

* The Kubernetes itself and its dependencies, like Docker. Gravity refers to this
  layer as "system software updates".
* The application(s) deployed inside, including auxiliary subsystems used for
  monitoring and logging. Special care must be taken around database
  migrations and the sequence in which various components are updated.

The Gravity update process is designed to update both layers. Here is how
Gravity update process works:

1. New versions of an application and the system binaries (container
   images) are downloaded and stored inside the cluster. This means that during the
   update, all data has already been saved locally and disruptions to external
   services, like Docker registries, will not affect the update process.

2. Gravity uses the Kubernetes [rolling update](https://kubernetes.io/docs/tasks/manage-daemon/update-daemon-set/)
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
Bundle onto a Gravity Cluster. Gravity supports this in both online and offline environments.

#### Online Cluster Update

If cluster is connected to an Ops Center and has remote support turned on, it can download
updated Application Bundles directly from that Ops Center.

Use the `gravity update download` command to automatically discover if there are new
versions available and download them:

```bsh
$ gravity update download              # Check and download any updates now.
$ gravity update download --every=12h  # Schedule automatic downloading of updates every 12 hours.
$ gravity update download --every=off  # Turn off automatic downloading of updates.
```

#### Offline Cluster Update

If a Gravity Cluster is offline or not connected to an Ops Center, the new version of the Application
Bundle has to be copied to one of the Application Cluster nodes and the Cluster nodes need to be accessible
to each other. To upload the new version, extract the tarball and launch the `upload` script.

### Performing Upgrade

Once a new Application Bundle has been uploaded into the Cluster, a new upgrade operation can be started.

An upgrade can be triggered either through web UI or from command line.
To trigger the upgrade from UI, select an appropriate version on the "Updates" tab click `Update`.

To trigger the operation from command line, extract the Application Bundle tarball to a directory:

```bash
$ cd installer
$ tar xf application-bundle.tar
$ ls -lh
total 64M
-rw-r--r--. 1 user user 1.1K Jan 14 09:08 README
-rw-r--r--. 1 user user  13K Jan 14 09:08 app.yaml
-rwxr-xr-x. 1 user user  63M Jan 14 09:08 gravity
-rw-------. 1 user user 256K Jan 14 09:08 gravity.db
-rwxr-xr-x. 1 user user  907 Jan 14 09:08 install
drwxr-xr-x. 5 user user 4.0K Jan 14 09:08 packages
-rwxr-xr-x. 1 user user  344 Jan 14 09:08 upgrade
-rwxr-xr-x. 1 user user  411 Jan 14 09:08 upload
```

Inside the directory, execute the `upgrade` script to upload the update and start the operation.

Alternatively, upload the update and execute the `gravity upgrade` command which provides more control:

```bash
$ sudo ./upload
Wed Jan 14 17:02:20 UTC	Importing application app v0.0.1-alpha.2
Wed Jan 14 17:02:24 UTC	Synchronizing application with Docker registry 172.28.128.1:5000
Wed Jan 14 17:02:53 UTC	Application has been uploaded
installer$ sudo ./gravity upgrade
```

Executing the command with `--no-block` will start the operation in background from a systemd service.

#### Manual Upgrade

If you specify `--manual | -m` flag, the operation is started in manual mode:

```bsh
installer$ sudo ./gravity upgrade --manual
updating app from 0.0.1-alpha.1 to 0.0.1-alpha.2
Deploying agents on nodes
The operation has been created in manual mode.

See https://gravitational.com/gravity/docs/cluster/#managing-an-ongoing-operation for details on working with operation plan.
```

Please refer to the [Managing an Ongoing Operation](cluster.md#managing-an-ongoing-operation) section about
working with the operation plan.

!!! tip
    Manual upgrade steps must be executed with the gravity binary included in the upgrade
    tarball to ensure version compatibility. If you don't have an installer tarball (for
    example, when downloading upgrades directly from connected Ops Center), you can obtain
    the appropriate gravity binary from the distribution Ops Center (see [Getting the Tools](quickstart.md#getting-the-tools)).


### Troubleshooting Automatic Upgrades

!!! tip "Advanced Usage"
    This section covers the "under the hood" details of the automatic updates.

When a user initiates an automatic update by executing `gravity upgrade`
command, the following actions take place:

1. An agent (update agent) is deployed on each cluster node. The
   update agents start on every cluster node as a `systemd` unit called
   `gravity-agent.service`.
1. The agents execute phases of the update plan. These are
   the same phases a user would run as part of a [manual upgrade](#manual-upgrade).
1. Once the update is complete, agents are shut down.

Below is the list of the low-level commands executed by `gravity upgrade`
to achieve this. These commands can also be executed manually
from a terminal on any master node in a Gravity Cluster:

```bsh
# Copy the update agent to every cluster node and start the agents:
root$ ./gravity agent deploy

# Run specific operation steps:
root$ ./gravity plan execute --phase=<phase>

# Alternatively, resume update from the last aborted step:
root$ ./gravity plan resume

# Shut down the update agents on all nodes:
root$ ./gravity agent shutdown
```

## Direct Upgrades From Older LTS Versions

Gravity LTS releases are at most 8 months apart and are based on Kubernetes releases which are no more than 2 minor versions apart.
This requirement is partially necessitated by the Kubernetes version skew [support policy](https://kubernetes.io/docs/setup/release/version-skew-policy/#supported-version-skew).
Gravity can thus only upgrade clusters from a previous LTS version. For example, an existing cluster based on Gravity `5.0.35` can be upgraded to one based on Gravity
`5.2.14` but not to `5.5.20`.

Since version `5.5.21` `tele` is capable of producing cluster image tarballs that can upgrade clusters based on older LTS versions (i.e. more than one version apart) directly.
For this to work, it embeds the data from (a series) of previous LTS releases.

As a result, the tarball will grow roughly by 1Gb per embedded release. Additionally, each embedded LTS release increases the upgrade time by a certain
amount (which is cluster size-specific) - this should also be taken into account when planning for the upgrade.

To embed an LTS release, specify its version as a parameter to `tele build`:

```bash
$ tele build ... --upgrade-via=5.2.15
```

The flag can be specified multiple times to add as many LTS versions as required.

!!! note "Embedding intermediate LTS releases"
    The version specified with the `--upgrade-via` flag must be an LTS version.
    Check [Releases](changelog.md) page to see which LTS versions are available for embedding.
    The upgrade path from the existing version must contain all intermediate LTS releases to reach the target version but
    the target version does not have to be LTS.
    For example, to upgrade a cluster based on Gravity `5.0.35` to the image based on Gravity `5.5.21`, the cluster
    image must embed the LTS version `5.2.15`.


## Managing An Ongoing Operation

Some operations in a Gravity cluster require cooperation from all cluster nodes.
Examples are installs, upgrades and garbage collection.
Additionally, due to varying complexity and unforeseen conditions, these operations can and do fail in practice.

To provide a foundation for coping with failures, the operations are built as sets of smaller steps
that can be re-executed or rolled back individually. This allows for interactive fix and retry loop should any of the
steps fail.

Each operation starts with building an operational plan - a tree of actions to perform in order
to achieve a particular goal. Once created, the plan is either executed automatically or manually step by step to completion.


!!! note
    Starting with `5.3.7-alpha.1`, operation plan management is conveniently available under the `gravity plan` command:

```bash
$ gravity plan --help
usage: gravity plan [<flags>] <command> [<args> ...]

Manage operation plan

Flags:
      --help                 Show context-sensitive help (also try --help-long and --help-man).
      --debug                Enable debug mode
  -q, --quiet                Suppress any extra output to stdout
      --insecure             Skip TLS verification
      --state-dir=STATE-DIR  Directory for local state
      --log-file="/var/log/telekube-install.log"
                             log file with diagnostic information

Subcommands:
  plan display* [<flags>]
    Display a plan for an ongoing operation

  plan execute [<flags>]
    Execute specified operation phase

  plan rollback [<flags>]
    Rollback specified operation phase

  plan resume [<flags>]
    Resume last aborted operation

  plan complete
    Mark operation as completed
```


### Displaying Operation Plan

In order to display an operation plan for the currently active operation:

```bash
$ sudo gravity plan
Phase                    Description                                                 State         Node              Requires                           Updated
-----                    -----------                                                 -----         ----              --------                           -------
* init                   Initialize update operation                                 Unstarted     -                 -                                  -
* checks                 Run preflight checks                                        Unstarted     -                 /init                              -
* bootstrap              Bootstrap update operation on nodes                         Unstarted     -                 /init                              -
  * node-1               Bootstrap node "node-1"                                     Unstarted     -                 -                                  -
* masters                Update master nodes                                         Unstarted     -                 /checks,/bootstrap,/pre-update     -
  * node-1               Update system software on master node "node-1"              Unstarted     -                 -                                  -
    * drain              Drain node "node-1"                                         Unstarted     172.28.128.1      -                                  -
    * system-upgrade     Update system software on node "node-1"                     Unstarted     -                 /masters/node-1/drain              -
    * taint              Taint node "node-1"                                         Unstarted     172.28.128.1      /masters/node-1/system-upgrade     -
    ...
* runtime                Update application runtime                                  Unstarted     -                 /masters                           -
  * rbac-app             Update system application "rbac-app" to 0.0.1-alpha.2       Unstarted     -                 -                                  -
  * site                 Update system application "site" to 0.0.1-alpha.2           Unstarted     -                 /runtime/rbac-app                  -
  * kubernetes           Update system application "kubernetes" to 0.0.1-alpha.2     Unstarted     -                 /runtime/rbac-app                  -
* app                    Update installed application                                Unstarted     -                 /masters,/runtime/rbac-app         -
  * telekube             Update application "telekube" to 0.0.1-alpha.2              Unstarted     -                 -                                  -
...
```

The command is aliased as `gravity plan display`.
The plan lists all steps, top to bottom, in the order in which they will be executed.
Each step (phase), has a state which explains whether it has already run or whether it failed.
Also, steps can explicitly or implicitly depend on other steps.
The commands will make sure a particular phase cannot be executed before its requirements have not run.

If a phase has failed, the `display` command will also show the corresponding error message.


### Executing Operation Plan

Remember that an operation plan is effectively a tree of steps. Whenever you need to execute a particular step,
you need to specify it as absolute path from a root node:

```bash
$ sudo gravity plan execute --phase=/masters/node-1/drain
```

Whole groups of steps can be executed if only the parent node has been specified as path.
For example, the following:

```bash
$ sudo gravity plan execute --phase=/masters
```

will execute all steps of the `/masters` node in the order listed.

Sometimes it is necessary to force execution of a particular step although it has already run.
To do this, add `--force` flag to the command line:

```bash
$ sudo gravity plan execute --phase=/masters/node-1/drain --force
```

If it is impossible to make progress with an operation due to an unforeseen condition, the
steps that have been executed to this point should be rolled back:

```bash
$ sudo gravity plan rollback --phase=/masters/node-1/taint
$ sudo gravity plan rollback --phase=/masters/node-1/system-upgrade
$ sudo gravity plan rollback --phase=/masters/node-1/drain
...
```

Note the reverse order of invocation.
And just like with execution, the steps can be rolled back in groups:

```bash
$ sudo gravity plan rollback --phase=/masters
```

Once all steps have been rolled back, the operation needs to be explicitly completed in order to mark it failed:

```bash
$ sudo gravity plan complete
```

If you have fixed and issue and would like to resume the operation:

```bash
$ sudo gravity plan resume
```

This will resume the operation at the last failed step and run it through to completion.
In this case there's no need to explicitly complete the operation afterwards - this is done
automatically upon success.


## Interacting with the Master Container

As explained [above](#kubernetes-environment), Gravity runs Kubernetes inside a master container.
The master container (sometimes called "planet") makes sure that every single
deployment of Gravity is the same and all nodes look identical to each other
from the inside.  The master container includes all Kubernetes components and
dependencies such as kube-apiserver, etcd and docker.

To launch an interactive shell to get a system view of a Gravity cluster, you
can use `gravity shell` command on a node running Gravity:

```bsh
$ sudo gravity shell
```

`graivty exec` command is quite similar to `docker exec`: it executes the
specified command inside the master container:

```bsh
# Allocate PTY with attached STDIN and launch the interactive bash inside the master container
$ sudo gravity exec -ti /bin/bash

# Execute an non-interactive command inside the master conatiner
$ sudo gravity exec /bin/ls
```

## Separation of Workloads

Clusters with complex software deployed on them require separation of workloads between the control plane and application components to enable a seamless upgrade experience.

Gravity release 4.23+ leverages the support for node taints and tolerations in Kubernetes. Taints are special labels that control which resources can be scheduled onto a node. A pod needs to tolerate a taint in order to be schedulable on that particular node.

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

With the new `preUpdate` hook, the Gravity Cluster scales down the application resources in preparation for the update. The scaling logic needs to be aware of the cluster size to make appropriate scaling decisions.
The hook execution is implemented as a separate phase and is executed automatically in automatic upgrade mode.

You can find out more about other hook types on the [Gravity documentation on Packaging and Deployment](pack.md#application-hooks).

#### Apply a system taint to the node

The node taint is required so that the node will only schedule the system applications after a restart. The user application pods will be scheduled once the taint has been removed (see below for the step that does this).

```bsh
$ sudo gravity upgrade --phase=/masters/<node-name>/taint
```

#### Drain the node to be updated

The node is drained before the system software on it is updated. Draining entails marking the node as unschedulable and evicting the running Pods (subject to Eviction API and PodDisruptionBudgets). The nodes are drained one by one to avoid disrupting too many resources.

This process serves multiple purposes:

- It helps the node to start in a clean state.
- It marks the resources as not running so that the scheduler and other controllers can make educated decisions.

```bsh
$ sudo gravity upgrade --phase=/masters/<node-name>/drain
```

#### Update system software on the node

This step entails updating the system software on the node.

```bsh
$ sudo gravity upgrade --phase=/masters/<node-name>
```

#### Make the node schedulable to start system applications

This is the reverse of the drain step. It removes the scheduling restriction and allows the scheduler to schedule pods on this node. However, the pods will only be scheduled if they tolerate the system taint.

This step is implemented as an additional phase:

```bsh
$ sudo gravity upgrade --phase=/masters/<node-name>/uncordon
```
#### Remove the system taint

This step removes the artificial restriction imposed by the taint step (see above) and allows the application pods to be scheduled on this node.

```bsh
$ sudo gravity upgrade --phase=/masters/<node-name>/untaint
```

#### Update the runtime

This step updates the runtime (system applications) and precedes the user application update.

```bsh
$ sudo gravity upgrade --phase=/runtime
```

#### Perform the application update

This step executes the user application update hooks, after which the application is considered updated.

```bsh
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
the Gravity Cluster.

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
`--state-dir` | _(Optional)_ Directory where all Gravity system data will be kept on this node. Defaults to `/var/lib/gravity`.

Every node in a Cluster must have a role, defined in the Application
Manifest. A role defines the system requirements for the node. For example, nodes
of "database" role must have storage attached to them. Gravity enforces the
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

```bsh
$ gravity leave --force
```

A node can also be removed from the Cluster records by running `gravit remove` on any
node in the Cluster.

```bsh
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

This command will update Etcd, Kubernetes and Gravity state to remove faulty node from the
database. Let's assume the remaining nodes are:

```bsh
1.2.3.5
1.2.3.6
```

We are going to add `1.2.3.7` to replace the faulty `1.2.3.4`.

On the node `1.2.3.5`, execute this command:

```bsh
gravity remove 1.2.3.4 --force
```

Wait until the operation completes by polling the status:

```bsh
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

```bsh
sudo gravity join 1.2.3.5 --advertise-addr=1.2.3.7 --token=<join token>
```

Make sure that operation is completed by executing `gravity status`:

```bsh
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

#### Auto Scaling the cluster

When running on AWS, Gravity integrates with [Systems manager parameter store](http://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-paramstore.html) to simplify the discovery.

Assuming the cluster name is known, a new node can join the cluster using `gravity autojoin` command

```bsh
sudo gravity autojoin example.com --role=knode
```

`gravity discover` will locate the cluster load balancer and join token by reading parameter store
and automatically connect. This command can be run as a part of `cloud-init` of the AWS instance.

Users can read more about AWS integration [here](https://github.com/gravitational/provisioner#provisioner)

## Backup And Restore

Gravity Clusters support backing up and restoring the application state. To enable backup
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
                image: quay.io/gravitational/debian-tall:stretch
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
                image: quay.io/gravitational/debian-tall:stretch
                command:
                  - "/usr/local/bin/kubectl"
                  - "apply"
                  - "-f"
                  - "/var/lib/gravity/backup/backup.yaml"
                  - "--namespace"
                  - "default"
```

To trigger a backup, log into one of the cluster nodes and execute:

```bsh
root$ gravity backup <data.tar.gz>
```

where `<data.tar.gz>` is the name of the output backup archive. The tarball contains
compressed contents of `/var/lib/gravity/backup` directory from the backup hook.

To restore the data from the backup tarball:

```bsh
root$ gravity restore <data.tar.gz>
```

!!! tip
    You can use `--follow` flag for backup/restore commands to stream hook logs to
    standard output.

## Garbage Collection

Every now and then, the cluster would accumulate resources it has no use for - be it Gravity
packages or docker images from previous versions of the application. These resources will unnecessarily
waste disk space and while possible are usually difficult to get rid of manually.

The `gravity` tool offers a subset of commands to run cluster-wide garbage collection.
During garbage collection, the following resources are pruned:

  * Unused Gravity packages from previous versions of the application
  * Unused docker images from previous versions of the application
  * Obsolete systemd journal directories

!!! node "Docker image pruning"
    The tool currently employs a simple approach to pruning docker images.
    It will remove all images and repopulate the registry from the application state
    so only the images that are necessary for the current version of the application
    and all of its dependencies are available in the registry.
    If you have custom images in the registry you will need to push them again
    after the garbage collection.

To start garbage collection, use the `gc` subcommand of the `gravity` tool:

```bsh
$ sudo gravity gc [--phase=PHASE] [--confirm] [--resume] [--manual]
```

If started without parameters (i.e. not in manual mode) the command will run the operation automatically.
After the operation has been started, the regular `gravity plan` can be used to display the plan of the
ongoing operation.

In case of any intermediate failures, the command will abort and print the corresponding error message.
After fixing the issue, the operation can be resumed with:

```bsh
$ sudo gravity gc --resume
```

To execute a specific phase:

```bsh
$ sudo gravity gc --phase=<PHASE>
```

!!! top "Completing manual operation"
    At the end of the manual or aborted operation, explicitly resume the operation to complete it.


## Remote Assistance

Every Gravity cluster can be connected to an Ops Center,
assuming the cluster is connected to the Internet. This creates an outbound SSH tunnel from the cluster
to the Ops Center and the operator can use that tunnel to perform remote troubleshooting
by using the `tsh` tool. You can read more about remote assistance in the
[remote management](manage.md) section.

However, some Gravity Cluster owners may want to disable the SSH tunnel and keep their
clusters disconnected from the vendor, only enabling this capability when they
need help.

Remote Assistance can be turned on or off in the web UI or by using the
`gravity tunnel` command:

```bsh
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
node to uninstall the application as well as all Gravity software and data.

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

```bsh
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

Gravity borrows the concept of resources from Kubernetes to configure itself.
Use `gravity resource` command to update the cluster configuration.

Currently supported resources are:

Resource Name             | Resource Description
--------------------------|---------------------
`oidc`                    | OIDC connector
`github`                  | GitHub connector
`saml`                    | SAML connector
`role`                    | cluster role
`user`                    | cluster user
`token`                   | user tokens such as API keys
`logforwarder`            | forwarding logs to a remote rsyslog server
`trusted_cluster`         | managing access to remote Ops Centers
`endpoints`               | Ops Center endpoints for user and cluster traffic
`cluster_auth_preference` | cluster authentication settings such as second-factor
`alert`                   | cluster monitoring alert
`alerttarget`             | cluster monitoring alert target
`smtp`                    | cluster monitoring SMTP configuration
`runtimeenvironment`      | cluster runtime environment variables
`clusterconfiguration`    | cluster configuration
`authgateway`             | authentication gateway configuration

### Configuring OpenID Connect

An Gravity Cluster can be configured to authenticate users using an
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
  redirect_url: "https://gravity-url/portalapi/v1/oidc/callback"
  client_id: <client id>
  client_secret: <client secret>
  issuer_url: "https://example.com/"
  scope: [roles]
  claims_to_roles:
    - {claim: "roles", value: "gravitational/admins", roles: ["@teleadmin"]}
```

Add this connector to the cluster:

```bsh
$ gravity resource create oidc.yaml
```

To list the installed connectors:

```bsh
$ gravity resource get oidc
```

To remove the connector `auth0`:

```bsh
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

### Configuring GitHub Connector

Gravity supports authentication and authorization via GitHub. To configure
it, create a YAML file with the resource spec based on the following example:

```yaml
kind: github
version: v3
metadata:
  name: example
spec:
  # Github OAuth app client ID
  client_id: <client-id>
  # Github OAuth app client secret
  client_secret: <client-secret>
  # Github will make a callback to this URL after successful authentication
  # cluster-url is the address the cluster UI is reachable at
  redirect_url: "https://<cluster-url>/portalapi/v1/github/callback"
  # connector display name that will be appended to the title of "Login with"
  # button on the cluster login screen so it will say "Login with Github"
  display: Github
  # mapping of Github team memberships to Gravity cluster roles
  teams_to_logins:
    - organization: example
      team: admins
      logins:
        - "@teleadmin"
```

Create the connector:

```bsh
$ gravity resource create github.yaml
```

Once the connector has been created, the cluster login screen will start
presenting "Login with GitHub" button.

!!! note
    When going through the Github authentication flow for the first time, the
    application must be granted the access to all organizations that are present
    in the "teams to logins" mapping, otherwise Gravity will not be able to
    determine team memberships for these organizations.

To view configured GitHub connectors:

```bsh
$ gravity resource get github
```

To remove a GitHub connector:

```bsh
$ gravity resource rm github example
```

### Configuring SAML Connector

Gravity supports authentication and authorization via SAML providers. To
configure it, create a YAML file with the resource spec based on the following example:

```yaml
kind: saml
version: v2
metadata:
  name: okta
spec:
  # SAML provider will make a callback to this URL after successful authentication
  # cluster-url is the address the cluster UI is reachable at
  acs: https://<cluster-url>/portalapi/v1/saml/callback
  # mapping of SAML attributes to Gravity roles
  attributes_to_roles:
    - name: groups
      value: admins
      roles:
        - @teleadmin
  # connector display name that will be appended to the title of "Login with"
  # button on the cluster login screen so it will say "Login with Okta"
  display: Okta
  # SAML app metadata in XML format downloaded from SAML provider
  entity_descriptor: |
    ...
```

!!! note
    For an example of configuring a SAML application with Okta take a look
    at the following guide: [SSH Authentication With Okta](https://gravitational.com/teleport/docs/ssh_okta/).

Create the connector:

```bsh
$ gravity resource create saml.yaml
```

To view configured SAML connectors:

```bsh
$ gravity resource get saml
```

To remove a SAML connector:

```bsh
$ gravity resource rm saml okta
```

### Configuring Roles

Below is an example of a resource file with the definition of an admin role. The admin has
access to all resources, including roles, other users and authentication settings such
as OIDC connectors, and belongs to a privileged Kubernetes group:

```yaml
kind: role
version: v3
metadata:
  name: administrator
spec:
  allow:
    kubernetes_groups:
    - admin
    logins:
    - root
    node_labels:
      '*': '*'
    rules:
    - resources:
      - '*'
      verbs:
      - '*'
  options:
    max_session_ttl: 30h0m0s
```

Below is an example of a non-admin role spec providing access to a particular
cluster `example.com` and its applications:

```yaml
kind: role
version: v3
metadata:
  name: developer
spec:
  allow:
    logins:
    - root
    node_labels:
      '*': '*'
    kubernetes_groups:
    - admin
    rules:
    - resources:
      - role
      verbs:
      - read
    - resources:
      - app
      verbs:
      - list
    - resources:
      - cluster
      verbs:
      - read
      - update
      where: equals(resource.metadata.name, "example.com")
  options:
    max_session_ttl: 10h0m0s
```

To create these two roles you can execute:

```bsh
$ gravity resource create administrator.yaml
$ gravity resource create developer.yaml
```

To view all currently available roles:

```bsh
$ gravity resource get role
```

To delete the `developer` role:

```bsh
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

```bsh
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

```bsh
$ gravity resource create token.yaml
```

To view available users and a user's tokens:

```bsh
$ gravity resource get user
$ gravity resource get token --user=alice@example.com
```

### Example: Provisioning A Publisher User

In this example we are going to use `role`, `user` and `token` resources described above to
provision a user who can publish applications into an Ops Center. For instructions on how
to setup your own Ops Center see [Setting up an Ops Center](opscenter.md).

In the following spec we define 3 resources:

1. role `publisher` that allows its holder to push applications
2. user `alice@example.com` that carries this role
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
    - resources:
      - repository
      verbs:
      - read
      - list
    - resources:
      - app
      verbs:
      - read
      - list
      - create
      - update
---
kind: user
version: v2
metadata:
  name: "alice@example.com"
spec:
  type: "agent"
  roles: ["publisher"]
---
kind: token
version: v2
metadata:
   name: "s3cr3t!"
spec:
   user: "alice@example.com"
```

Save the resources into `publisher.yaml` and create them on the cluster:

```bsh
$ gravity resource create publisher.yaml
```

This is how the new user can publish new application bundles into the Ops Center:

```bsh
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

```bsh
$ gravity resource create admin.yaml
```

The new user can now log into the cluster via the Web UI with the user
credentials created above.

!!! tip "Password Restrictions"
    Passwords must be between 6 and 128 characters long.

### Configuring Log Forwarders

Every Gravity Cluster is automatically set up to aggregate the logs from all
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

```bsh
$ gravity resource create forwarder.yaml
```

To view currently configured log forwarders, run:

```bsh
$ gravity resource get logforwarders
```

To delete a log forwarder:

```bsh
$ gravity resource rm logforwarder forwarder1
```

### Configuring TLS Key Pair

Ops Center and Gravity Cluster Web UI and API TLS key pair can be configured
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

```bsh
$ gravity resource create tlskeypair.yaml
```

To view currently configured key pair:

```bsh
$ gravity resource get tls
```

To delete a TLS key pair (in this case default self-signed TLS key pair will be used instead):

```bsh
$ gravity resource rm tls keypair
```

### Configuring Trusted Clusters

!!! note
    Support for trusted clusters is available since Gravity version
    `5.0.0-alpha.5`.

Trusted clusters is a concept for connecting standalone Gravity clusters to
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
in the [Installing Ops Center](opscenter.md#installing-ops-center) flow (the first
port).

Create the trusted cluster:

```bsh
$ gravity resource create trustedcluster.yaml
```

View the currently configured trusted cluster:

```bsh
$ gravity resource get trusted_cluster
Name                      Enabled     Pull Updates     Reverse Tunnel Address          Proxy Address
----                      -------     ------------     ----------------------          -------------
opscenter.example.com     true        true             opscenter.example.com:3024      opscenter.example.com:32009
```

Once the cluster has been created, the reverse tunnel status can be viewed and
managed using `gravity tunnel` shortcut commands:

```bsh
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

```bsh
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

```bsh
$ gravity resource create endpoints.yaml
```

!!! note
    Updating the endpoints resource will result in restart of `gravity-site`
    pods so the changes can take effect.

To view currently configured endpoints, run:

```bsh
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

```bsh
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

```bsh
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

```bsh
# kubectl get services -n kube-system -l app=gravity-opscenter
NAME             TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)                         AGE
gravity-public   LoadBalancer   10.100.20.71    <pending>     443:31792/TCP,3023:32083/TCP    59m
gravity-agents   LoadBalancer   10.100.91.204   <pending>     4443:30873/TCP,3024:30185/TCP   8s
```

### Configuring Cluster Authentication Gateway

!!! note
    Authentication gateway resource is supported starting Gravity version `5.5.0`.

Cluster authentication gateway handles authentication/authorization and allows
users to remotely access the cluster nodes via SSH or Kubernetes API.

To tweak authentication gateway configuration use the following resource:

```yaml
kind: authgateway
version: v1
spec:
  # Connection throttling settings
  connection_limits:
    # Max number of simultaneous connections
    max_connections: 1000
    # Max number of simultaneously connected users
    max_users: 250
  # Cluster authentication preferences
  authentication:
    # Auth type, can be "local", "oidc", "saml" or "github"
    type: oidc
    # Second factor auth type, can be "off", "otp" or "u2f"
    second_factor: otp
    # Default auth connector name
    connector_name: google
  # Determines if SSH sessions to cluster nodes are forcefully terminated
  # after no activity from a client, for example "30m", "1h", "1h30m"
  client_idle_timeout: never
  # Determines if the clients will be forcefully disconnected when their
  # certificates expire in the middle of an active SSH session
  disconnect_expired_cert: false
  # DNS name that applies to all SSH, Kubernetes and web proxy endpoints
  public_addr:
    - example.com
  # DNS name of the gateway SSH proxy endpoint, overrides "public_addr"
  ssh_public_addr:
    - ssh.example.com
  # DNS name of the gateway Kubernetes proxy endpoint, overrides "public_addr"
  kubernetes_public_addr:
    - k8s.example.com
  # DNS name of the gateway web proxy endpoint, overrides "public_addr"
  web_public_addr:
    - web.example.com
```

To update authentication gateway configuration, run:

```bash
$ gravity resource create gateway.yaml
```

!!! note
    The `gravity-site` pods will be restarted upon resource creation in order
    for the new settings to take effect, so the cluster management UI / API
    will become briefly unavailable.

When authentication gateway resource is created, only settings that were
explicitly set are applied to the current configuration. For example, to
only limit the maximum number of connections, you can create the following
resource:

```yaml
kind: authgateway
version: v1
spec:
  connection_limits:
    max_conections: 1500
```

The following command will display current authentication gateway configuration:

```bash
$ gravity resource get authgateway
```

### Configuring Cluster Authentication Preference

!!! warning "Deprecation warning"
    Cluster authentication preference resource is obsolete starting Gravity
    version `5.5.0` and will be removed in a future version. Please use
    [Authentication Gateway](cluster.md#configuring-cluster-authentication-gateway)
    resource instead.

Cluster authentication preference resource allows to configure method of
authentication users will use when logging into a Gravity cluster.

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
  # default authentication connector to use for tele login
  connector_name: google
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

```bsh
$ gravity resource create auth.yaml
```

!!! note
    Make sure to configure a proper [OIDC connector](cluster.md#configuring-openid-connect)
    when using "oidc" authentication type.

To view the currently configured authentication preference:

```bsh
$ gravity resource get cluster_auth_preference
Type      ConnectorName     SecondFactor
----      -------------     ------------
local                       off
```

### Configuring Monitoring

See [Kapacitor Integration](monitoring.md#kapacitor-integration) about details
on how to configure monitoring alerts.

### Configuring Runtime Environment Variables

In a Gravity cluster, each node is running a runtime container that hosts Kubernetes.
All services (including Kubernetes native services like API server or kubelet) execute with the predefined
environment (set up during installation or update).
If you need to make changes to the runtime environment, i.e. introduce new environment variables
like `HTTP_PROXY`, this resource will allow you to do that.

To add a new environment variable, `HTTP_PROXY`, create a file with following contents:

[envars.yaml]
```yaml
kind: RuntimeEnvironment
version: v1
spec:
  data:
    HTTP_PROXY: "example.com:8001"
```

To install a cluster with the new runtime environment, specify the resources file as an argument
to the `install` command:

```bsh
$ sudo gravity install --cluster=<cluster-name> --config=envars.yaml
```

On an installed cluster, create the resource with:

```bash
$ sudo gravity resource create -f envars.yaml
Updating cluster runtime environment requires restart of runtime containers on all nodes.
The operation might take several minutes to complete depending on the cluster size.

The operation will start automatically once you approve it.
If you want to review the operation plan first or execute it manually step by step,
run the operation in manual mode by specifying '--manual' flag.

Are you sure?
confirm (yes/no):
yes
```

Without additional parameters, the operation is executed automatically, but can be placed into manual mode with
the specification of `--manual | -m` flag to the `gravity resource`  command:

```bash
$ sudo gravity resource create -f envars.yaml --manual
```

This will allow you to control every aspect of the operation as it executes.
See [Managing an Ongoing Operation](cluster.md#managing-an-ongoing-operation) for more details.


To view the currently configured runtime environment variables:

```bash
$ gravity resource get runtimeenvironment
Environment
-----------
HTTP_PROXY=example.com:8081
```

To remove the configured runtime environment variables, run:

```bash
$ gravity resource rm runtimeenvironment
```

!!! warning
    Adding or removing cluster runtime environment variables is disruptive as it necessitates the restart
    of runtime containers on each cluster node. Take this into account and plan each update accordingly.


### Cluster Configuration

It is possible to customize the cluster per environment before the installation or update some aspects of the cluster
using the `ClusterConfiguration` resource:

[cluster-config.yaml]
```yaml
kind: ClusterConfiguration
version: v1
spec:
  global:
    # configures the cloud provider
    cloudProvider: gce
    # free-form cloud configuration
    cloudConfig: |
      multizone=true
      gce-node-tags=demo-cluster
    # represents the IP range from which to assign service cluster IPs
    serviceCIDR:  "10.0.0.0/24"
    # port range to reserve for services with NodePort visibility
    serviceNodePortRange: "30000-32767"
    # host port range (begin-end, single port or begin+offset, inclusive) that
    # may be consumed in order to proxy service traffic
    proxyPortRange: "0-0"
    # CIDR range for Pods in cluster
    podCIDR: "10.0.0.0/24"
    # A set of key=value pairs that describe feature gates for alpha/experimental features
    featureGates:
      AllAlpha: true
      APIResponseCompression: false
      BoundServiceAccountTokenVolume: false
      ExperimentalHostUserNamespaceDefaulting: true
  # kubelet configuration as described here: https://kubernetes.io/docs/tasks/administer-cluster/kubelet-config-file/
  # and here: https://github.com/kubernetes/kubelet/blob/release-1.13/config/v1beta1/types.go#L62
  kubelet:
    config:
      kind: KubeletConfiguration
      apiVersion: kubelet.config.k8s.io/v1beta1
      nodeLeaseDurationSeconds: 50
```

In order to apply the configuration immediately after the installation, supply the configuration file
to the `gravity install` command:

```bsh
root$ ./gravity install --cluster=<cluster-name> ... --config=cluster-config.yaml
```

!!! note
    You can combine multiple kubernetes and Gravity-specific resources in the config file prior to
    running the install command to have the installer automatically create all resources upon installation.

!!! warning
    Setting feature gates overrides value set by the runtime container by default.


In order to update configuration of an active cluster, use the `gravity resource` command:

```bsh
root$ ./gravity resource create cluster-config.yaml
```

The operation can be started in manual mode in which case you have the ability to review the operation
plan or cancel the operation. To put the operation into manual mode, use the `--manual` flag:

```bsh
root$ ./gravity resource create cluster-config.yaml --manual
```

The configuration update is implemented as a cluster operation. Once created, it is managed using
the same `gravity plan` command described in the [Managing an Ongoing Operation](cluster.md#managing-an-ongoing-operation) section.


To view the configuration:

```bsh
root$ ./gravity resource get config
```

To remove (reset to defaults) the configuration:

```bsh
root$ ./gravity resource rm config
```


!!! warning
    Updating the configuration of an active cluster is disruptive and might necessitate the restart
    of runtime containers either on master or on all cluster nodes. Take this into account and plan
    each update accordingly.


## Managing Users

Gravity cluster allows to invite new users and reset passwords for existing
users by executing CLI commands on the cluster nodes.

### Invite User

To invite a user execute the `gravity users add <username>` command on one of
the cluster nodes. The command accepts the following flags:

Flag      | Description
----------|-------------
`--roles` | List of roles to assign to a new user. The built-in role `@teleadmin` grants admin permissions.
`--ttl`   | Time to live (TTL) for the invite token. Examples: "5h", "10m", "1h30m", The default is "8h", maximum is "48h".

The command will generate a signup URL valid for the specified amount of time:

```bsh
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

```bsh
$ gravity users reset alice@example.com --ttl=1h
Password reset token has been created and is valid for 1h0m0s. Share this URL with the user:
https://<host>/web/reset/<token>
```

!!! note
    Make sure that `<host>` is accessible to the user.

## Securing a Cluster

Gravity comes with a set of roles and bindings (for role-based access control or RBAC) and a set of pod security policies. This lays the ground for further security configurations.

### Pod security policies

Introduced in Kubernetes 1.5, Pod security policies allow controlled access to privileged containers based on user roles and groups. A Pod security policy specifies what a Pod can do and what it has access to.
Policies allow the administrator to control many facets of the system (See [PodSecurityPolicies] for details).

By default Gravity provides two security policies: `privileged` and `restricted`.

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

```bsh
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

```bsh
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

By default Gravity:

  * connects default service accounts (in `default` and `kube-system` namespaces) to the built-in `cluster-admin` role
  * creates `admin`/`edit`/`view` Gravity specific groups (bound to respective built-in cluster roles)

See the Kubernetes [RBAC] documentation for more information.


## Eviction Policies

Kubernetes includes support for evicting pods in order to maintain node stability, which is documented [here](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).

Gravity uses the following eviction policies by default:

 - Hard eviction
    1. less than 5% of disk space is available (`nodefs.available<5%`)
    1. less than 5% of the inodes are available (`nodefs.inodesFree<5%`)
    1. less than 5% of the image filesystem is free (`imagefs.available<5%`)
    1. less than 5% of the inodes on the image filesystem are available (`imagefs.inodesFree<5%`)

 - Soft eviction
    1. less than 10% of disk space is available (`nodefs.available<10%`)
    1. less than 10% of the inodes are available (`nodefs.inodesFree<10%`)
    1. less than 10% of the image filesystem is free (`imagefs.available<10%`)
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

Gravity supports provisioning nodes with [Taints](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) to control
workload distribution. Use `taints` property of the node profiles:

```yaml
nodeProfiles:
  - name: node
    description: "Gravity Node"
    taints:
    - key: custom-taint
      value: custom-value
      effect: NoExecute
```

### Dedicated control plane nodes

To install Kubernetes nodes running only system components, Gravity supports `node-role.kubernetes.io/master` taint:

```yaml
nodeProfiles:
  - name: master
    description: "Gravity Master"
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
    description: "Gravity Master Node"
    labels:
      node-role.kubernetes.io/master: "true"
```

If the node is has been assigned the `node-role.kubernetes.io/node: "true"` label, it will be set up as a Kubernetes node,
i.e. will run Etcd configured in proxy mode:

```yaml
nodeProfiles:
  - name: node
    description: "Gravity Node"
    labels:
      node-role.kubernetes.io/node: "true"
```

If none of the labels above are set, Gravity will automatically assign node roles according to the following algorithm:

  * If there are already 3 master nodes available (either explicitly set via labels or already installed/elected
    in the system) - assign as kubernetes node.
  * Otherwise promote the node to a Kubernetes master.


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


### WireGuard Encrypted Networking

Gravity supports encrypting all pod-to-pod traffic between hosts using [WireGuard](https://www.wireguard.com) to create
an encrypted VPN between hosts. This feature is configured through the application manifest when building Gravity applications:

```yaml
providers:
  generic:
    network:
      type: wireguard
```

!!! tip "Version"
    The WireGuard feature is only available from 5.5.0-alpha.3 and later

!!! tip "Kernel module"
    The WireGuard feature currently requires the WireGuard kernel module to be installed and available on the host. Please see
    the [WiregGuard installation instructions](https://www.wireguard.com/install/) for more information.

## Customizing Cluster DNS

Gravity uses [CoreDNS](https://coredns.io) for DNS resolution and service discovery within the cluster.

The CoreDNS configuration can be edited after installation, by updating the `kube-system/coredns` ConfigMap.


[//]: # (Footnotes and references)

[PodSecurityPolicies]: https://kubernetes.io/docs/concepts/policy/pod-security-policy/
[RBAC]: https://kubernetes.io/docs/admin/authorization/rbac/
[promiscuous-mode]: https://en.wikipedia.org/wiki/Promiscuous_mode
