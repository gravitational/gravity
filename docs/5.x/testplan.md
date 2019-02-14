## Manual Test Plan

### Preparation

- [ ] Build `opscenter` and `telekube` installers off your branch: `make production opscenter telekube` should do it.

### Ops Center

- [ ] Install Ops Center in CLI mode.
  - [ ] Verify can configure [OIDC connector](https://gravitational.com/gravity/docs/ver/5.x/cluster/#google-oidc-connector-example), for example:
```yaml
kind: oidc
version: v2
metadata:
  name: google
spec:
  redirect_url: "https://<ops-advertise-addr>/portalapi/v1/oidc/callback"
  client_id: <cliend-id>
  client_secret: <client-secret>
  issuer_url: https://accounts.google.com
  scope: [email]
  claims_to_roles:
    - {claim: "hd", value: "gravitational.com", roles: ["@teleadmin"]}
```
  - [ ] Verify can log into Ops Center UI.
  - [ ] Verify can update TLS certificate via [resource](https://gravitational.com/gravity/docs/ver/5.x/cluster/#configuring-tls-key-pair) or UI.
  - [ ] Verify can log in with `tele login`.
  - [ ] Verify can push Telekube app into Ops Center.
  - [ ] Verify can invite user to Ops Center using CLI.
    - [ ] Create a user invite: `gravity users add test@example.com --roles=@teleadmin`.
    - [ ] Open the generated link and signup.
    - [ ] Verify can login with the created user.
  - [ ] Verify can reset Ops Center user password using CLI.
    - [ ] Request a user reset: `gravity users reset test@example.com`.
    - [ ] Open the generated link and reset the password.
    - [ ] Verify can login with the new password.

### Standalone Telekube

#### CLI mode

- [ ] Install Telekube application in standalone CLI mode.
  - [ ] Verify can create [local user](https://gravitational.com/gravity/docs/ver/5.x/cluster/#example-provisioning-a-cluster-admin-user), for example:
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
  - [ ] Verify can connect to [Ops Center](https://gravitational.com/gravity/docs/ver/5.x/cluster/#configuring-trusted-clusters).
    - [ ] Verify cluster appears as online in Ops Center and can be accessed via UI.
    - [ ] Verify remote support can be toggled off/on and cluster goes offline/online respectively.
    - [ ] Verify trusted cluster can be deleted and cluster disappears from Ops Center.
  - [ ] Verify can join a node using CLI (`gravity join`).
  - [ ] Verify can remove a node using CLI (`gravity leave`).

#### UI mode

- [ ] Install Telekube application in standalone UI wizard mode.
  - [ ] Verify can complete bandwagon through wizard UI.
  - [ ] Verify can log into local cluster UI with the user created in bandwagon.
  - [ ] Verify can connect to [Ops Center](https://gravitational.com/gravity/docs/ver/5.x/cluster/#configuring-trusted-clusters).
    - [ ] Verify cluster appears as online in Ops Center and can be accessed via UI.
    - [ ] Verify remote support can be toggled off/on and cluster goes offline/online respectively.
    - [ ] Verify trusted cluster can be deleted and cluster disappears from Ops Center.

### Via Ops Center

#### Via UI

- [ ] Install Telekube application via Ops Center.
  - [ ] Verify can complete bandwagon.
  - [ ] Verify can log into local cluster UI with the user created in bandwagon.
  - [ ] Verify cluster is connected to the Ops Center.
    - [ ] Verify remote support is configured and turned on: cluster appears "online" in Ops Center.
    - [ ] Verify remote support can be toggled off/on and cluster goes offline/online respectively.
    - [ ] Verify trusted cluster can be deleted/created and cluster disappears from/appears in Ops Center respectively.
  - [ ] Verify can `tele login` into the installed cluster via Ops Center.
    - [ ] Verify can use tsh, e.g. `tsh clusters` or `tsh ls`.
    - [ ] Verify can use kubectl, e.g. `kubectl get nodes`.
  - [ ] Verify can join a node using UI.
  - [ ] Verify periodic updates.
    - [ ] Enable periodic updates on the cluster: `gravity update download --every=1m`.
    - [ ] Verify cluster checks for updates by looking at the logs.

#### With Installer

- [ ] Install Telekube application using installer downloaded from Ops Center, CLI or UI.
  - [ ] Verify cluster connects back to Ops Center after installation.
    - [ ] Verify remote support is configured but turned off: cluster appears "offline" in Ops Center.
    - [ ] Verify remote support can be toggled on/off and cluster goes online/offline respectively.
    - [ ] Verify trusted cluster can be deleted/created and cluster disappears from/appears in Ops Center respectively.

### AWS

#### Via Ops Center Using Automatic Provisioner

 - [ ] Install Ops Center on AWS.
 - [ ] Configure DNS, OIDC connector and push Telekube app.
 - [ ] Install 3-node cluster on AWS using automatic provisioner via Ops Center.
    - [ ] Verify cluster is connected to the Ops Center.
     - [ ] Verify remote support can be toggled off/on and cluster goes offline/online respectively.
     - [ ] Verify trusted cluster can be deleted/created and cluster disappears from/appears in Ops Center respectively.
   - [ ] Verify can `tele login` into the installed cluster via Ops Center.
     - [ ] Verify can use tsh, e.g. `tsh clusters` or `tsh ls`.
     - [ ] Verify can use kubectl, e.g. `kubectl get nodes`.
   - [ ] Verify can join a node.
   - [ ] Verify can uninstall the cluster.
     - [ ] Verify AWS instances and other resources are deprovisioned.

### Failover & Resiliency

- [ ] Install 3-node cluster.
  - [ ] Shutdown currently active master node (let's say it's `node-1`).
    - [ ] Verify that another node was elected as master and all relevant Kubernetes services are running.
    - [ ] Verify that `kubectl` commands keep working.
    - [ ] Verify that `gravity status` is reporting the cluster as degraded.
  - [ ] Remove the shutdown node from the cluster by executing `gravity remove node-1 --force` from one of the remaining healthy nodes.
    - [ ] Verify that `node-1` is successfully removed from the cluster.
    - [ ] Verify that `gravity status` is reporting the cluster as healthy (may take a minute for it to recover).

### Ops Center / Cluster Upgrade & Connectivity

- [ ] Install an Ops Center of previous LTS version.
  - [ ] Push Telekube app of previous LTS version into it.
  - [ ] Install a single-node Telekube cluster.
- [ ] Upgrade Ops Center to the current version.
  - [ ] Verify the cluster stays connected & online.
  - [ ] Verify remote support can be toggled off/on.
- [ ] Push Telekube app of the current version to the Ops Center.
- [ ] Upgrade the cluster to the current version.
  - [ ] Verify the cluster stays connected & online.
  - [ ] Verify remote support can be toggled off/on.

### Cluster Upgrade & Join

- [ ] Install a 1-node cluster of previous LTS version.
- [ ] Upgrade the cluster to the current version.
- [ ] Join another node to the cluster.
  - [ ] Verify the node joined successfully.

### Tele Build

#### Open-Source Edition

- [ ] Create minimal app manifest (`app.yaml`):
```yaml
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
    name: test
    resourceVersion: 1.0.0
```

- [ ] Verify can build installer:
```bash
$ tele build app.yaml
```
  - [ ] Verify the latest compatible runtime from `hub.gravitational.io` was selected:
  ```bash
  $ tele ls --with-prereleases
  ```

- [ ] Pin runtime in the manifest to some version compatible with tele (same major/minor version components):
```yaml
systemOptions:
    runtime:
        version: 5.2.0
```
  - [ ] Verify can build the installer.

#### Enterprise Edition

- [ ] Run the same tests as for OSS version.
  - [ ] Verify `get.gravitational.io` instead of `hub.gravitational.io` was used as a remote repository.

- [ ] Log into some Ops Center (could be local dev one).
```bash
$ tele login -o example.gravitational.io
```

- [ ] Unpin runtime from manifest and run tele build.
  - [ ] Verify latest compatible runtime from active Ops Center was selected.

### Application Catalog

This section covers the application catalog features. It requires an Ops Center.

#### Building & Publishing

- [ ] Build application images from the sample Helm charts in `assets/charts`.
```bash
$ tele build assets/charts/alpine-0.1.0
$ tele build assets/charts/alpine-0.2.0
```

- [ ] Log into the Ops Center.
```bash
$ tele login -o ops.gravitational.io:32009
```

- [ ] Push the built application image into the Ops Center.
```bash
$ tele push alpine-0.1.0.tar
```

- [ ] Make sure the application is shown in the image list.
```bash
$ tele ls
```

- [ ] Verify the application image tarball can be downloaded from the Ops Center.
```bash
$ tele pull alpine:0.1.0
```

- [ ] Verify Helm chart can be searched for and downloaded from the Ops Center.
```bash
$ helm search alpine
$ helm fetch ops.gravitational.io/alpine --version 0.1.0
```

- [ ] Verify one of the application's Docker images can be pulled from the Ops Center.
```bash
$ docker pull ops.gravitational.io:32009/alpine:3.3
```

#### Discovery

- [ ] Install a cluster.

- [ ] Connect the cluster to the Ops Center using [Trusted Cluster](https://gravitational.com/gravity/docs/cluster/#configuring-trusted-clusters) resource.

- [ ] Verify the application can be searched for in the connected Ops Center.
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

- [ ] Install the application directly from the Ops Center.
```bash
$ gravity app install <opscenter-name>/alpine:0.1.0
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

### Licensing & Encryption (Enterprise Edition)

This scenario builds an encrypted installer for an application that requires
a license and makes sure that it can be installed with valid license. It is
only supported in the enterprise edition.

- [ ] Generate test CA and private key:
```bash
$ openssl req -newkey rsa:2048 -nodes -keyout domain.key -x509 -days 365 -out domain.crt
```

- [ ] Create test app manifest that requires license (`app.yaml`):
```yaml
apiVersion: bundle.gravitational.io/v2
kind: Bundle
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

- [ ] Build an encrypted application installer:
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
