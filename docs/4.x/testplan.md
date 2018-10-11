## Manual Test Plan

### Preparation

- [ ] Build `opscenter` and `telekube` installers off your branch: `make production opscenter telekube` should do it.

### Ops Center

- [ ] Install Ops Center in CLI mode.
  - [ ] Verify can configure [OIDC connector](https://gravitational.com/gravity/docs/ver/4.x/cluster/#google-oidc-connector-example), for example:
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
  - [ ] Verify can update TLS certificate via [resource](https://gravitational.com/gravity/docs/ver/4.x/cluster/#configuring-tls-key-pair) or UI.
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
  - [ ] Verify can create [local user](https://gravitational.com/gravity/docs/ver/4.x/cluster/#example-provisioning-a-cluster-admin-user), for example:
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
  - [ ] Verify can connect to [Ops Center](https://gravitational.com/gravity/docs/ver/4.x/cluster/#configuring-trusted-clusters).
    - [ ] Verify cluster appears as online in Ops Center and can be accessed via UI.
    - [ ] Verify remote support can be toggled off/on and cluster goes offline/online respectively.
    - [ ] Verify trusted cluster can be deleted and cluster disappears from Ops Center.
  - [ ] Verify can join a node using CLI (`gravity join`).
  - [ ] Verify can remove a node using CLI (`gravity leave`).

#### UI mode

- [ ] Install Telekube application in standalone UI wizard mode.
  - [ ] Verify can complete bandwagon through wizard UI.
  - [ ] Verify can log into local cluster UI with the user created in bandwagon.
  - [ ] Verify can connect to [Ops Center](https://gravitational.com/gravity/docs/ver/4.x/cluster/#configuring-trusted-clusters).
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

#### Using One-Time Link

- [ ] Install Telekube application via one-time install link generated by Ops Center.

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

#### With Installer Using Automatic Provisioner

- [ ] Prepare AWS node as described in [AWS Installer](https://gravitational.com/gravity/docs/ver/4.x/cluster/#aws-installer).
- [ ] Transfer Telekube installer on the node, unpack, define the cluster resource, for example:
```yaml
kind: cluster
version: v2
metadata:
  name: example.com
spec:
  provider: aws
  aws:
    region: us-east-2
    keyName: ops
  nodes:
  - profile: node
    count: 3
    instanceType: m4.xlarge
```
- [ ] Install the cluster: `./gravity install --cluster-spec=cluster.yaml`.
  - [ ] Verify cluster information is displayed at the end of the install.
  - [ ] Verify installed cluster is healthy.
  - [ ] Verify local Ops Center has been torn down.
