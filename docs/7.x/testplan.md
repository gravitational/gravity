## Manual Test Plan

### Preparation

- [ ] Build `opscenter` and `telekube` cluster images off your branch: `make production opscenter telekube`.

### Hub

- [ ] Install Hub in CLI mode.
  - [ ] Verify can configure [OIDC connector](https://gravitational.com/gravity/docs/ver/6.x/config/#example-google-oidc-connector), for example:
```yaml
kind: oidc
version: v2
metadata:
  name: google
spec:
  redirect_url: "https://<hub-advertise-addr>/portalapi/v1/oidc/callback"
  client_id: <cliend-id>
  client_secret: <client-secret>
  issuer_url: https://accounts.google.com
  scope: [email]
  claims_to_roles:
    - {claim: "hd", value: "gravitational.com", roles: ["@teleadmin"]}
```
  - [ ] Verify can log into the Hub UI.
  - [ ] Verify can update TLS certificate via [resource](https://gravitational.com/gravity/docs/ver/6.x/config/#tls-key-pair) or UI.
  - [ ] Verify can log in with `tele login`.
  - [ ] Verify can push Telekube cluster image into the Hub.
  - [ ] Verify can invite user to the Hub using CLI.
    - [ ] Create a user invite: `gravity users add test@example.com --roles=@teleadmin`.
    - [ ] Open the generated link and signup.
    - [ ] Verify can login with the created user.
  - [ ] Verify can reset the Hub user password using CLI.
    - [ ] Request a user reset: `gravity users reset test@example.com`.
    - [ ] Open the generated link and reset the password.
    - [ ] Verify can login with the new password.

### Standalone Cluster

#### CLI mode

- [ ] Install Telekube cluster image in standalone CLI mode.
  - [ ] Verify can create [local user](https://gravitational.com/gravity/docs/ver/6.x/config/#example-provisioning-a-cluster-admin-user), for example:
```yaml
kind: user
version: v2
metadata:
  name: admin@example.com
spec:
  type: admin
  roles: ["@teleadmin"]
  password: qwe123
```
  - [ ] Verify can invite user to cluster using CLI.
    - [ ] Create a user invite: `gravity users add test@example.com --roles=@teleadmin`.
    - [ ] Open the generated link and signup.
    - [ ] Verify can login with the created user.
  - [ ] Verify can reset cluster user password using CLI.
    - [ ] Request a user reset: `gravity users reset test@example.com`.
    - [ ] Open the generated link and reset the password.
    - [ ] Verify can login with the new password.
  - [ ] Verify can log into local cluster UI using the user created above.
  - [ ] Verify can join a node using CLI (`gravity join`).
  - [ ] Verify can remove a node using CLI (`gravity leave`).

#### UI mode

- [ ] Install Telekube cluster image in standalone UI wizard mode.
  - [ ] Verify can complete bandwagon through wizard UI.
  - [ ] Verify can log into local cluster UI with the user created in bandwagon.

#### With Cluster Image Downloaded From The Hub

- [ ] Install Telekube cluster image using installer downloaded from the Hub: CLI or UI.
  - [ ] Verify cluster connects back to the Hub after installation.
    - [ ] Verify remote support is configured but turned off: cluster appears "offline" in the Hub.

### Remote Support & Teleport Connectivity

- [ ] Install Telekube cluster image using any method.
- [ ] Create a local user using any method.

- [ ] Log into the cluster UI with the created user.
  - [ ] Verify can SSH into a cluster node using web terminal.

- [ ] Log into the cluster using `tsh` with the created user: `tsh login --proxy=<node>:32009 --user=<user>`.
  - [ ] Verify `tsh status`, `tsh ls` and `tsh ssh` commands work.
  - [ ] Verify `kubectl` was configured to talk to the cluster, e.g. `kubectl get nodes`, `kubectl get pods --all-namespaces`.

- [ ] Connect the cluster to the Hub via a [trusted cluster](https://gravitational.com/gravity/docs/ver/6.x/config/#trusted-clusters-enterprise) resource.
  - [ ] Verify cluster appears as online in the Hub UI and the cluster's UI can be accessed.
  - [ ] Verify can SSH into a cluster node using web terminal in the Hub UI.

- [ ] Log into the cluster via the Hub: `tele login --hub example.gravitational.io <cluster>`.
  - [ ] Verify `tsh status`, `tsh ls` and `tsh ssh` commands work.
  - [ ] Verify `kubectl` was configured to talk to the cluster, e.g. `kubectl get nodes`, `kubectl get pods --all-namespaces`.

- [ ] Turn off remote support on the cluster: `gravity tunnel disable`.
  - [ ] Verify the cluster appears offline in the Hub and can't be accessed anymore.

- [ ] Turn the remote support back on: `gravity tunnel enable`.
  - [ ] Verify the cluster is online again and can be accessed.

- [ ] Disconnect the cluster from the Hub: `gravity resource rm trustedcluster <hub>`.
  - [ ] Verify the cluster is removed from the Hub.

### Failover & Resiliency

- [ ] Install 3-node cluster.
  - [ ] Stop the planet systemd service on the active master node (let's say it's `node-1`).
    - [ ] Verify that another node was elected as master and all relevant Kubernetes services are running.
    - [ ] Verify that `kubectl` commands keep working.
    - [ ] Verify that `gravity status` is reporting the cluster as degraded.
    - [ ] Verify can still SSH onto `node-1` via Teleport using cluster UI.
  - [ ] Shutdown `node-1` completely.
  - [ ] Remove the shutdown node from the cluster by executing `gravity remove node-1 --force` from one of the remaining healthy nodes.
    - [ ] Verify that `node-1` is successfully removed from the cluster.
    - [ ] Verify that `gravity status` is reporting the cluster as healthy (may take a minute for it to recover).

### Hub / Cluster Upgrade & Connectivity

- [ ] Install a Hub of the previous LTS version.
  - [ ] Push Telekube app of the previous LTS version into it.
  - [ ] Install a single-node Telekube cluster and connect it to the Hub.
- [ ] Push Telekube app of the current version to the Hub.
- [ ] Upgrade the cluster to the current version.
  - [ ] Verify the cluster stays connected & online.
  - [ ] Verify remote support can be toggled off/on.
- [ ] Upgrade the Hub to the current version.
  - [ ] Verify the cluster stays connected & online.
  - [ ] Verify remote support can be toggled off/on.

### Cluster Upgrade & Join

- [ ] Install a 1-node cluster of previous LTS version.
- [ ] Upgrade the cluster to the current version.
- [ ] Join another node to the cluster.
  - [ ] Verify the node joined successfully.

### Tele Build

#### Open-Source Edition

- [ ] Create a minimal cluster image manifest (`app.yaml`):
```yaml
apiVersion: cluster.gravitational.io/v2
kind: Cluster
metadata:
    name: test
    resourceVersion: 1.0.0
```

- [ ] Verify `tele` selects base image matching its own version:
```bash
$ tele build app.yaml
```

- [ ] Pick base image version from the hub compatible with `tele` (same major/minor version components):
```bash
$ tele ls --all
```

- [ ] Pin base image in the manifest to the selected version:
```yaml
apiVersion: cluster.gravitational.io/v2
kind: Cluster
baseImage: gravity:6.0.0
metadata:
    name: test
    resourceVersion: 1.0.0
```

  - [ ] Verify can build the installer.
```bash
$ tele build app.yaml
```

#### Enterprise Edition

- [ ] Run the same tests as for OSS version.
  - [ ] Verify `get.gravitational.io` instead of `hub.gravitational.io` was used as a remote repository.

- [ ] Log into a Hub (could be local dev one).
```bash
$ tele login --hub example.gravitational.io
```

- [ ] Unpin runtime from manifest and run `tele build`.
  - [ ] Verify `tele` selects base image matching its own version.

### Application Catalog

This section covers the application catalog features. It requires a Hub.

#### Building & Publishing

- [ ] Build application images from the sample Helm charts in `assets/charts`.
```bash
$ tele build assets/charts/alpine-0.1.0
$ tele build assets/charts/alpine-0.2.0
```

- [ ] Log into the Hub.
```bash
$ tele login --hub example.gravitational.io:32009
```

- [ ] Push the built application image into the Hub.
```bash
$ tele push alpine-0.1.0.tar
```

- [ ] Make sure the application is shown in the image list.
```bash
$ tele ls
```

- [ ] Verify the application image tarball can be downloaded from the Hub.
```bash
$ tele pull alpine:0.1.0
```

- [ ] Verify Helm chart can be searched for and downloaded from the Hub.
```bash
$ helm search alpine
$ helm fetch example.gravitational.io/alpine --version 0.1.0
```

- [ ] Verify one of the application's Docker images can be pulled from the Hub.
```bash
$ docker pull example.gravitational.io:32009/alpine:3.3
```

#### Discovery

- [ ] Install a cluster.

- [ ] Connect the cluster to a Hub using [Trusted Cluster](https://gravitational.com/gravity/docs/config/#trusted-clusters-enterprise) resource.

- [ ] Verify the application can be searched for in the connected Hub.
```bash
$ gravity app search --all
$ gravity app search alpine --all
```

#### Lifecycle

- [ ] Transfer both built application images (`alpine-0.1.0.tar`, `alpine-0.2.0.tar`) onto the cluster node.

- [ ] Install the application.
```bash
$ gravity app install alpine-0.1.0.tar
```

- [ ] Verify an application instance has been deployed.
```bash
$ gravity app ls
$ kubectl get pods
```

- [ ] Install the application directly from the Hub.
```bash
$ gravity app install <hub-name>/alpine:0.1.0
```

- [ ] Verify there are now 2 instances of the application running.
```bash
$ gravity app ls
$ kubectl get pods
```

- [ ] Upgrade one of the deployed application.
```bash
$ gravity app upgrade <release-name> alpine-0.2.0.tar
```

- [ ] Verify the application has been upgraded.
```bash
$ gravity app ls
```

- [ ] Rollback the upgraded application.
```bash
$ gravity app rollback <release-name> 1
```

- [ ] Verify the application has been rolled back.
```bash
$ gravity app ls
```

### Licensing & Encryption [Enterprise Edition]

This scenario builds an encrypted installer for a cluster image that requires
a license and makes sure that it can be installed with valid license. It is
only supported in the enterprise edition.

- [ ] Generate test CA and private key:
```bash
$ openssl req -newkey rsa:2048 -nodes -keyout domain.key -x509 -days 365 -out domain.crt
```

- [ ] Create test app manifest that requires license (`app.yaml`):
```yaml
apiVersion: cluster.gravitational.io/v2
kind: Cluster
metadata:
    name: test
    resourceVersion: 1.0.0
license:
    enabled: true
```

- [ ] Generate a license with encryption key:
```bash
$ gravity license new --max-nodes=3 --valid-for=24h --ca-cert=domain.crt --ca-key=domain.key --encryption-key=qwe123 > license.pem
```

- [ ] Build an encrypted cluster image:
```bash
$ tele build app.yaml --ca-cert=domain.crt --encryption-key=qwe123
```

- [ ] Verify can install in wizard UI mode.
  - [ ] Verify license prompt appears in the UI.
  - [ ] Insert the generated license and verify the installation succeeds.
  - [ ] Verify license can be updated via cluster UI after installation.

- [ ] Verify license is enforced in CLI mode:
```bash
$ sudo ./gravity install # should return a license error
```

- [ ] Verify can install in CLI mode with license:
```bash
$ sudo ./gravity install --license="$(cat /tmp/license)"
```

### Runtime Environment Update

This scenario updates the runtime environment of the planet container with new environment variables.

Prerequisites: multi-node cluster with at least 1 node `--role=knode` and 1 `--role=node|master`.
Having both `knode` and `node|master` is necessary to test both master and regular node update paths.

[environ.yaml]
```yaml
kind: RuntimeEnvironment
version: v1
spec:
  data:
    "FOO": "qux"
    "BAR": "foobar"
```

```bash
root$ gravity resource create environ.yaml --confirm
```

- [ ] Verify the operation completes successfully.
  - [ ] Verify the environment inside the container has been updated on each node (the environment variables have been added to the /etc/container-environment).
  - [ ] Verify that services have the environment applied. Choose `dockerd` process for verification and check that `/proc/$(pidof dockerd)/environ` contains the configured environment variables.

### Cluster Configuration Update

This scenario updates the cluster configuration.

Prerequisites: multi-node cluster with at least 1 node `--role=knode` and 1 `--role=node|master`.
Having both `knode` and `node|master` is necessary to test both master and regular node update paths.

[config.yaml]
```yaml
kind: ClusterConfiguration
version: v1
spec:
  kubelet:
    config:
      kind: KubeletConfiguration
      apiVersion: kubelet.config.k8s.io/v1beta1
      # Update kubelet configuration
      nodeLeaseDurationSeconds: 50
      # Cannot update certain fields
      healthzBindAddress: "127.0.0.2"
      healthzPort: 11248
  global:
    featureGates:
      AllAlpha: true
      APIResponseCompression: false
      BoundServiceAccountTokenVolume: false
      # Update feature gates
      ExperimentalHostUserNamespaceDefaulting: true
```

```bash
root$ gravity resource create config.yaml --confirm
```

- [ ] Verify the operation completes successfully.
  - [ ] Verify the feature gates parameter of all kubernetes components has been updated to the configured set (`KUBE_COMPONENT_FLAGS` in /etc/container-environment inside the container).
  - [ ] Verify kubelet configuration (`/etc/kubernetes/kubelet.yaml`) reflects the configuration (`nodeLeaseDurationSeconds` is set to `50`)
  - [ ] Verify kubelet configuration has not been changed for fields `healthzBindAddress` (should still be "0.0.0.0") and `healthzPort` (should still be `10248`)

[cloud-config.yaml]
```yaml
kind: ClusterConfiguration
version: v1
spec:
  global:
    cloudConfig: |
      [global]
      # Update node tags
      # Should only work if installed with a cloud-provider
      node-tags=test-cluster
```

Now, create the operation in manual mode:

```bash
root$ gravity resource create cloud-config.yaml --confirm -m
```

- [ ] Verify that the operation plan only contains update steps for master nodes as only cloud configuration is being updated.
- [ ] Verify can complete the operation successfully with `gravity plan resume`.
  - [ ] Verify cloud configuration file has been written in `/etc/kubernetes/cloud-config.conf` with the following contents:
  ```
  [global]
  node-tags=test-cluster
  ```

### Collecting Garbage

This scenario tests garbage collection on a cluster.

Prerequisites: multi-node cluster with at least 1 node `--role=knode` and 1 `--role=node|master`.
Having both `knode` and `node|master` is necessary to test both master and regular node update paths.

Install a previous LTS version.

- [ ] Gather baseline pre-upgrade state:
  - `sudo du -sh /var/lib/gravity/{local/packages,site/packages,planet/registry}`
  - `sudo gravity package list | cut -f1-3 -d ' '`
  - `sudo gravity exec gravity package list --ops-url=https://gravity-site.kube-system.svc.cluster.local:3009 --insecure | cut -f1-3 -d ' '`

Upgrade to the release under test.

- [ ] Verify journal logs have been pruned [gravity.e#3429](https://github.com/gravitational/gravity.e/issues/3429)
  - `sudo du -h /var/lib/gravity/planet/log/journal` should show only one subdir.

- [ ] Gather pre-garbage-collection state.
  - `sudo du -sh /var/lib/gravity/{local/packages,site/packages,planet/registry}`
  - `sudo gravity package list | cut -f1-3 -d ' '`
  - `sudo gravity exec gravity package list --ops-url=https://gravity-site.kube-system.svc.cluster.local:3009 --insecure | cut -f1-3 -d ' '`
  - `sudo find /var/lib/gravity/planet/registry/ -path '*tags/*' -type d | egrep 'tags/[^/]+$' | sort`
  - `sudo gravity system gc package --dry-run`


Execute `gravity gc`.

- [ ] Verify the gc operation completes successfully.
  - `sudo gravity plan` (`--operation-id` may be looked up from `gravity status` or `gravity resource get operations`)
- [ ] Verify packages from the previous installation have been removed locally.
  - `sudo gravity package list | cut -f1-3 -d ' '` Diff with output from before upgrade.
  - `sudo du -sh /var/lib/gravity/local/packages` should show substantially lower usage (similar to baseline).
- [ ] Verify packages from the previous installation have been removed from cluster package storage.
  - `sudo gravity exec gravity package list --ops-url=https://gravity-site.kube-system.svc.cluster.local:3009 --insecure | cut -f1-3 -d ' '`
  - `sudo du -sh /var/lib/gravity/site/packages` should show substantially lower usage (similar to baseline).
- [ ] Verify packages from the current installation are still present.
  - `sudo gravity package list | cut -f1-3 -d ' '` Diff with output from before garbage collection.
  - `sudo gravity status` should be 'active' without any warnings.
- [ ] Verify old tags are no longer present in the registry.
  - `sudo find /var/lib/gravity/planet/registry/ -path '*tags/*' -type d | egrep 'tags/[^/]+$' | sort` Diff with output from before garbage collection.

## WEB UI

### Gravity Cluster

#### Side Nav
- [ ] Verify that company logo is shown with a product version.
- [ ] Verify that Gravity logo is shown when company logo is missing.

#### Top Nav
- [ ] Verify that status indicator reflects these cluster states: healthy/processing/failed.
- [ ] Verify that cluster public URL is shown.
- [ ] Verify that "Info View" button opens "Cluster Information" dialog.

  Cluster Information
  - [ ] Verify that public URL, internal URLs, and login command are shown.
  - [ ] Verify that "copy" button copies the text to the clipboard.

- [ ] Verify that user menu has "Logout" and "Account" options for local use.
- [ ] Verify that user menu has only "Logout" option for SSO user.

#### Dashboard
- [ ] Verify that "CPU Usage" shows current CPU data.
- [ ] Verify that "RAM Usage" shows current RAM data.
- [ ] Verify that "Usage Over Time" shows the last 60 seconds of CPU/RAM data.
- [ ] Verify that all charts show real-time data. You can open a terminal and try running `ls -R /` for a while.
- [ ] Verify that "Audit Logs" table shows recent events (today).
- [ ] Verify that "Operation" table shows cluster operations.

  Start a terminal session
  - [ ] Verify that terminal session appears in the "Operation" table.

- [ ] Verify that Application table shows all installed applications.

#### Nodes
- [ ] Verify that "Nodes" table shows all joined nodes.
- [ ] Verify that login dropdown shows a list of available logins.
- [ ] Verify that clicking on the login opens a terminal.
- [ ] Verify that "Add Node" button opens a dialog with instructions.
- [ ] Verify that "Add Node" dialog shows valid instructions per selected role.

#### Terminal Session
- [ ] Verify that input/output works as expected by typing on the keyboard.
- [ ] Verify that window resize works (use `yum -y install mc` to install midnight commander)

  SCP
  - [ ] Verify that Upload works.
  - [ ] Verify that Upload handles invalid paths and network errors.
  - [ ] Verify that Download works.
  - [ ] Verify that Download handles invalid paths and network errors.

#### Logs
- [ ] Verify that Logs are shown.

  Log forwarder settings
  - [ ] Verify that Creating/Deleting/Editing a log forwarder works.

#### Audit Logs
- [ ] Verify that Audit events of different types are shown.
- [ ] Verify that "Details" button opens a dialog with JSON.
- [ ] Verify that "Today" and "7 Days" options correctly filter events.
- [ ] Verify that "Custom" option opens a date picker dialog.

#### Roles
- [ ] Verify that roles are shown.
- [ ] Verify that "Create New Role" dialog works.
- [ ] Verify that deleting and editing a role works.
- [ ] Verify that error is displayed when saving invalid input.

#### Users
- [ ] Verify that users are shown.
- [ ] Verify that creating a new user works.
- [ ] Verify that editing user roles works.
- [ ] Verify that removing a user works.

  Reset Password
  - [ ] Verify that "Reset Password" dialog shows a reset link.

  Invite User
  - [ ] Verify that role dropdown shows all existing roles.
  - [ ] Verify that clicking on "Create invite link" shows an invite URL.

#### Auth Connectors
- [ ] Verify that creating OIDC/SAML/GITHUB connectors works.
- [ ] Verify that editing  OIDC/SAML/GITHUB connectors works.

  Templates
  - [ ] Verify that "New OIDC connector" dialog has OIDC template.
  - [ ] Verify that "New SAML connector" dialog has SAML template.
  - [ ] Verify that "New SAML connector" dialog has GITHUB template.

  Card Icons
  - [ ] Verify that GITHUB card has github icon
  - [ ] Verify that SAML card has SAML icon
  - [ ] Verify that OIDC card has OIDC icon

#### HTTPS Certificate
- [ ] Verify that it shows certificate details.
- [ ] Verify that updating a certificate works (try submitting invalid file formats).

#### Kubernetes
- [ ] Verify namespace selector.

  Config maps
  - [ ] Verify that configs maps are shown.
  - [ ] Verify that editing config maps with multiple files works. Should see multiple tabs and "unsaved" changes indicator.

  Pods
  - [ ] Verify that pods of different statuses are correctly displayed. Failed should be in red.
  - [ ] Verify that string search on table columns works.

    Container menu
    - [ ] Verify that "View Logs" works.
    - [ ] Verify that SSH to container works.

    Action menu
    - [ ] Verify that "Details" display the JSON dialog.
    - [ ] Verify that "Monitoring" opens the monitoring screen with selected pod charts.
    - [ ] Verify that "Logs" opens the logs screen with selected pod logs.

  Services
  - [ ] Verify that services are displayed.
  - [ ] Verify that "Details" opens a dialog with JSON.

  Jobs
  - [ ] Verify that jobs are displayed.
  - [ ] Verify that "Details" opens a dialog with JSON.

  Daemon Sets
  - [ ] Verify that daemon sets are displayed.
  - [ ] Verify that "Details" opens a dialog with JSON.

  Deployments
  - [ ] Verify that deployments are displayed.
  - [ ] Verify that "Details" opens a dialog with JSON.

#### Monitoring
- [ ] Verify that Grafana charts work.

#### Account
- [ ] Verify that changing a password works with 2FA disabled.
- [ ] Verify that changing a password works with 2FA enabled.

#### License
  Make sure that Cluster has a license, then navigate to the license screen via top right corner menu (logout menu)
- [ ] Verify that license details are displayed.
- [ ] Verify that updating a license works.


#### Extensions
Set the following settings in the manifest file
```
app.manifest.extensions.monitoring.disabled: true
app.manifest.extensions.kubernetes.disabled: true
app.manifest.extensions.logs.disabled: true
```
 - [ ] Verify that Kubernetes, Logs, and Monitoring features are hidden.


### Invite Form
- [ ] Verify that company logo is shown.
- [ ] Verify input validation.
- [ ] Verify that invite works with 2FA disabled.
- [ ] Verify that invite works with OTP enabled.
- [ ] Verify that invite works with U2F enabled.
- [ ] Verify that error message is shown if an invite is expired/invalid.

### Login Form
- [ ] Verify that company logo is shown.
- [ ] Verify input validation.
- [ ] Verify that login works with 2FA disabled.
- [ ] Verify that login works with OTP enabled.
- [ ] Verify that login works with U2F enabled.
- [ ] Verify that SSO login works for Github/SAML/OIDC.
- [ ] Verify that account is locked after several unsuccessful attempts.

### GRAVITY HUB

#### Clusters
- [ ] Verify that 2 types of cluster cards (bundle and cluster) are shown.
- [ ] Verify that empty indicator is shown when there are no clusters.

  Cluster Card
  - [ ] Verify that bundle card displays icons of installed application.
  - [ ] Verify that location indicator works (AWS or Onprem).
  - [ ] Verify that cluster labels are displayed.
  - [ ] Verify cluster status indicator (green/yellow/red).

#### Catalog
- [ ] Verify that app images are displayed.
- [ ] Verify that empty indicator is shown when there are no images.

  App Card
- [ ] Verify that dropdown with versions works.
- [ ] Verify that cluster image has the right icon.
- [ ] Verify that bundle image has the right icon.
- [ ] Verify install instructions for cluster and bundle images.
- [ ] Verify that download button works.

#### Licenses
- [ ] Verify that license generator works.
- [ ] Verify input validation by entering invalid values.

#### User/Auth
  For Users, Roles, and Auth Connectors, please use Cluster steps to verify this functionality.

#### Settings
  For HTTP Cerifticate and for Account, please use Cluster steps to verify this functionality.

### Installer
  Create a cluster image which requires EULA, a valid License, and has Bandwagon step.
  Start an installer.

#### EULA
- [ ] Verify that EULA agreement is shown asking a user to accept it.

#### Step. License
- [ ] Verify that "License" step is shown.
- [ ] Verify that step indicator has the correct number of steps (5).
- [ ] Verify that error is shown when entering invalid license.

#### Step. Name You cluster
- [ ] Verify that cluster name is required.

  Additional options
  - [ ] Verify that input validation for subnet values works.
  - [ ] Verify that creating cluster labels works.

#### Step. Capacity
- [ ] Verify that profile selector works.
- [ ] Verify that input fields for IP Address and Mounts work (check input validation).
- [ ] Verify that "VERIFY" button works (should show a notification banner on top).

#### Step. Progress
- [ ] Verify that progress indicator works.
- [ ] Verify that installation logs work.
- [ ] Verify that when installation fails, the error is shown with "DOWNLOAD TARBALL" button.

#### Step. Create Admin
- [ ] Verify that creating an admin user works. After submitting a form, a user should be redirected to the cluster UI.

### RBAC
 Create the following role
```
  kind: role
  metadata:
    name: no_access
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
        - cluster
        - role
        verbs:
        - read
    deny: {}
    options:
  version: v3
```
  - [ ] Verify that a user has access only to: Nodes, Logs, Operations.
  - [ ] Verify that Nodes screen does not display k8s labels.

#### Add Role `list` access.
```
    - resources:
      - role
      verbs:
      - read
```
  - [ ] Verify that a list of roles is visible.
  - [ ] Verify that changing/creating a role is not allowed.

#### Add Role `update` access.
```
    - resources:
      - role
      verbs:
      - update
```
  - [ ] Verify that changing/creating a role is allowed.

#### Add Event `list` access.
```
    - resources:
      - event
      verbs:
      - list
```
  - [ ] Verify that list of events is displayed. In Event tab and on the Dashboard.

#### Add Log forwarder `list` access.
  ```
    - resources:
      - logforwarder
      verbs:
      - list
  ```
  - [ ] Verify that a list of log forwarders is visible.
  - [ ] Verify that changing/creating a log forwarder is not allowed.

#### Add Log forwarder `update` access.
```
    - resources:
      - logforwarder
      verbs:
      - update
```
  - [ ] Verify that changing/creating a log forwarder is allowed.

#### Add Auth Connector `list` access.
```
    - resources:
      - auth_connector
      verbs:
      - list
```
  - [ ] Verify that a list of auth connectors is visible.
  - [ ] Verify that changing/creating an auth connector is not allowed.

#### Add auth_connector `update` access.
```
    - resources:
      - auth_connector
      verbs:
      - update
```
  - [ ] Verify that changing/creating an auth connector is allowed.

#### Add app `list` access.
```
    - resources:
      - app
      verbs:
      - list
```
  - [ ] Verify that list of apps is displayed in Gravity HUB.


#### Add Cluster `connect` access.
```
    - resources:
      - cluster
      verbs:
      - connect
```
  - [ ] Verify that K8s tab is displayed.
  - [ ] Verify that Nodes screen displays K8s labels.
