# Releases

## Current Releases

Every major Gravity version `x.0.0` has it's long term support release, e.g. for `3.0.0` version
LTS starts with `3.51.0` with minor backwards compatible changes added over time until the end of support cycle.

| Release       | LTS | Release Date         | Supported Until      | Kubernetes Version | Teleport Version |
| --------------|-----| -------------------- | -------------------- | ------------------ |------------------|
| 6.3.6         | No  | February 4th, 2020   | -                    | 1.17.0             | 3.2.13           |
| 6.2.5*        | No  | December 3rd, 2019   | -                    | 1.16.3             | 3.2.13           |
| 6.1.18        | Yes | February 4th, 2020   | November 10th, 2021  | 1.15.5             | 3.2.12           |
| 6.0.10*       | No  | October 17th, 2019   | -                    | 1.14.7             | 3.2.12           |
| 5.6.8*        | No  | September 18th, 2019 | -                    | 1.14.7             | 3.0.6-gravity    |
| 5.5.38        | Yes | March 10th, 2020     | September 7th, 2020  | 1.13.11            | 3.0.6-gravity    |
| 5.4.10*       | No  | March 26th, 2019     | -                    | 1.13.5             | 2.4.10           |
| 5.3.9*        | No  | March 7th, 2019      | -                    | 1.12.3             | 2.4.7            |
| 5.2.16        | Yes | October 11th, 2019   | October 15th, 2019   | 1.11.9             | 2.4.10           |
| 5.0.35        | Yes | September 2nd, 2019  | April 13th, 2019     | 1.9.13-gravitational | 2.4.10         |
| 4.68.0*       | Yes | January 17th, 2019   | November 16th, 2018  | 1.7.18-gravitational | 2.3.5          |
| 3.64.0*       | Yes | December 21st, 2017  | June 2nd, 2018       | 1.5.7              | 2.0.6            |
| 1.30.0*       | Yes | March 21st, 2017     | March 21st, 2018     | 1.3.8              | 1.2.0            |

!!! tip "Cluster certificates expiration"
    If you have a Gravity cluster of version before `5.0.0-alpha.12` that
    hasn't been upgraded in about a year, its certificates may be expiring soon.
    If you are unable to upgrade, or your cluster certificates have already
    expired, please refer to the [Gravity Manual Certificates Renewal](https://gravitational.zendesk.com/hc/en-us/articles/360000755967-Telekube-Manual-Certificates-Renewal)
    article in our Help Center.

!!! note "Unsupported releases"
    Releases marked with `*` in the table above are no longer supported and
    do not receive updates and bugfixes.

!!! note "Direct upgrades"
    You can now upgrade existing 5.0.x clusters directly to 5.5.x.
    See [Direct Upgrades From Older LTS Versions](cluster.md#direct-upgrades-from-older-lts-versions) for details.

## 6.x Releases

### 6.3.6 (February 4th, 2020)

#### Bugfixes

* Fix an issue with flannel incorrectly recovering from a watch failure ([#1070](https://github.com/gravitational/gravity/pull/1070)).
* Fix an issue with changes to pod CIDR within cluster configuration ([#1043](https://github.com/gravitational/gravity/pull/1043)).
* Fix broken menu height and scrollbars ([#1042](https://github.com/gravitational/gravity/pull/1042)).
* Fix a UI issue with null items returned by kubernetes API ([#1039](https://github.com/gravitational/gravity/pull/1039)).
* Enable all kubernetes default admission controllers ([#1069](https://github.com/gravitational/gravity/pull/1069)).

### 6.1.18 LTS (February 4th, 2020)

#### Improvements

* Make username/password for SMTP configuration optional ([#1062](https://github.com/gravitational/gravity/pull/1062)).

#### Bugfixes

* Fix an issue with flannel incorrectly recovering from a watch failure ([#1070](https://github.com/gravitational/gravity/pull/1070)).
* Enable all kubernetes default admission controllers ([#1070](https://github.com/gravitational/gravity/pull/1070)).


### 6.3.5 (January 16th, 2020)

#### Bugfixes

* Fix the issue with gravity-site sometimes failing to start with bad permissions error ([#1024](https://github.com/gravitational/gravity/pull/1024)).

### 6.3.4 (January 14th, 2020)

#### Improvements

* Add Amazon Linux 2 to supported distros of the base Gravity image ([#1019](https://github.com/gravitational/gravity/pull/1019)).

#### Bugfixes

* Fix the issue with "role not found" error when trying to access remote clusters via Gravity Hub ([#1012](https://github.com/gravitational/gravity/pull/1012)).

### 6.1.16 LTS (January 14th, 2020)

#### Improvements

* Add Amazon Linux 2 to supported distros of the base Gravity image ([#1018](https://github.com/gravitational/gravity/pull/1018)).

#### Bugfixes

* Fix the issue with Gravity Hub installation ([#994](https://github.com/gravitational/gravity/pull/994)).
* Restore automatic node registration via kubelet ([#1014](https://github.com/gravitational/gravity/pull/1014), [planet#541](https://github.com/gravitational/planet/pull/541)).
* Fix the issue with "role not found" error when trying to access remote clusters via Gravity Hub ([#1010](https://github.com/gravitational/gravity/pull/1010)).

### 6.3.3 (January 8th, 2020)

#### Bugfixes

* Restore automatic node registration via kubelet ([#1001](https://github.com/gravitational/gravity/pull/1001), [planet#539](https://github.com/gravitational/planet/pull/539)).

### 6.3.2 (December 20th, 2019)

#### Bugfixes

* Fix an issue with CoreDNS pods not being scheduled due to discrepancy between node selector and node labels ([#985](https://github.com/gravitational/gravity/pull/985)).

### 6.1.15 LTS (December 20th, 2019)

#### Bugfixes

* Fix an issue with CoreDNS pods not being scheduled due to discrepancy between node selector and node labels ([#986](https://github.com/gravitational/gravity/pull/986)).

### 6.3.1 (December 20th, 2019)

#### Bugfixes

* Fix a security issue where secrets are being reused for multiple certificates and secrets are not being rotated during certificate rotation [#979](https://github.com/gravitational/gravity/pull/979).

!!! warning
    This release fixes a security vulnerability in gravity.

### 6.1.14 LTS (December 20th, 2019)

#### Improvements

* Add ability to override Helm values during tele build [#909](https://github.com/gravitational/gravity/pull/909).

#### Bugfixes

* Fix a security issue where secrets are being reused for multiple certificates and secrets are not being rotated during certificate rotation [#980](https://github.com/gravitational/gravity/pull/980).
* Fix an issue where node labels specified within the app.yaml could prevent cluster installation [#954](https://github.com/gravitational/gravity/pull/954).
* Prevent deletion of the base app [#966](https://github.com/gravitational/gravity/pull/966).
* Fix a performance issue with displaying cluster endpoints [#952](https://github.com/gravitational/gravity/pull/952)
* Fix a display issue with how the join command is displayed [#933](https://github.com/gravitational/gravity/pull/933).
* Expose the teleport kubernetes proxy as part of the gravity-site load balancer [#913](https://github.com/gravitational/gravity/pull/913).

!!! warning
    This release fixes a security vulnerability in gravity.

### 6.3.0 (December 18th, 2019)

#### Improvements

* Upgrade Kubernetes to `v1.17.0` ([#967](https://github.com/gravitational/gravity/pull/967), [planet#537](https://github.com/gravitational/planet/pull/537)).
* Remove Docker brigde and promiscuous mode configurations ([#959](https://github.com/gravitational/gravity/pull/959), [planet#536](https://github.com/gravitational/planet/pull/536)).
* Use relative binary path when displaying `gravity join` command hint ([#935](https://github.com/gravitational/gravity/pull/935)).

#### Bugfixes

* Fix the issue with kubelet failing to start with unsupported labels ([#953](https://github.com/gravitational/gravity/pull/953)).
* Fix the issue with `gravity status` becoming slow when there are a lot of namespaces ([#956](https://github.com/gravitational/gravity/pull/956)).
* Fix the issue with disconnecting clusters from the Hub ([#964](https://github.com/gravitational/gravity/pull/964)).

### 6.2.5 (December 3rd, 2019)

#### Improvements

* Add ability to pass Helm values to `tele build` ([#912](https://github.com/gravitational/gravity/pull/912)).

#### Bugfixes

* Expose Kubernetes proxy port in `gravity-public` service ([#916](https://github.com/gravitational/gravity/pull/916)).

### 6.2.4 (November 20th, 2019)

#### Improvements

* Display cluster name on a separate line in `gravity status` ([#896](https://github.com/gravitational/gravity/pull/896)).

#### Bugfixes

* Fix the issue with `gravity leave --force` leaving the node in partially cleaned up state when run without `sudo` ([#896](https://github.com/gravitational/gravity/pull/896)).
* Fix the issue with joining agent connecting to the ongoing installation instead of waiting for install to complete ([#893](https://github.com/gravitational/gravity/pull/893)).

### 6.1.13 LTS (November 20th, 2019)

#### Improvements

* Display cluster name on a separate line in `gravity status` ([#895](https://github.com/gravitational/gravity/pull/895)).

#### Bugfixes

* Fix the issue with `gravity leave --force` leaving the node in partially cleaned up state when run without `sudo` ([#895](https://github.com/gravitational/gravity/pull/895)).
* Fix the issue with joining agent connecting to the ongoing installation instead of waiting for install to complete ([#885](https://github.com/gravitational/gravity/pull/885)).

### 6.2.3 (November 13th, 2019)

#### Improvements

* Upgrade Kubernetes to `v1.16.3` ([#878](https://github.com/gravitational/gravity/pull/878), [planet#528](https://github.com/gravitational/planet/pull/528)).
* Execute preflight checks during join operation ([#854](https://github.com/gravitational/gravity/pull/854)).
* Update `gravity check` command to support upgrade preflight checks ([#871](https://github.com/gravitational/gravity/pull/871)).
* Bump Helm/Tiller to `v2.14.3` ([#830](https://github.com/gravitational/gravity/pull/830)).

#### Bugfixes

* Fix the issue with accessing remote clusters via a Hub using `tsh` or web terminal ([#816](https://github.com/gravitational/gravity/pull/816)).
* Fix the issue with the installer systemd unit failing due to long command when installing with a `--license` flag ([#831](https://github.com/gravitational/gravity/pull/831)).
* Fix the issue with application-only (without runtime) upgrades ([#836](https://github.com/gravitational/gravity/pull/836)).

### 6.1.12 LTS (November 11th, 2019)

#### Improvements

* Execute preflight checks during join operation ([#853](https://github.com/gravitational/gravity/pull/853)).
* Update `gravity check` command to support upgrade preflight checks ([#870](https://github.com/gravitational/gravity/pull/870)).
* Disable unused Docker bridge interface ([#873](https://github.com/gravitational/gravity/pull/873),  [planet#527](https://github.com/gravitational/planet/pull/527)).

### 6.1.11 (October 31st, 2019)

#### Bugfixes

* Fix the issue with cluster erroneously returning to the active state after upgrade operation failure ([#846](https://github.com/gravitational/gravity/pull/846), [#857](https://github.com/gravitational/gravity/pull/857)).

### 6.1.10 (October 24th, 2019)

#### Bugfixes

* Fix the issue with the installer systemd unit failing due to long command when installing with a `--license` flag ([#821](https://github.com/gravitational/gravity/pull/821)).
* Fix the issue with application-only (without runtime) upgrades ([#835](https://github.com/gravitational/gravity/pull/835)).

### 6.1.9 (October 21st, 2019)

#### Bugfixes

* Fix the issue with accessing remote clusters via a Hub using `tsh` or web terminal ([#814](https://github.com/gravitational/gravity/pull/814)).
* Fix the issue with `tiller` server not being upgraded when upgrading from early 6.1 releases ([#818](https://github.com/gravitational/gravity/pull/818)).

### 6.2.2 (October 17th, 2019)

#### Bugfixes

* Upgrade Kubernetes to `v1.16.02` (CVE-2019-11253) ([#808](https://github.com/gravitational/gravity/pull/808)).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://github.com/kubernetes/kubernetes/issues/83253) for more information.

### 6.1.8 (October 17th, 2019)

#### Bugfixes

* Upgrade Kubernetes to `v1.15.05` (CVE-2019-11253) ([#809](https://github.com/gravitational/gravity/pull/809)).
* Fix an issue with upgrades related to fetching legacy teleport configuration ([#809](https://github.com/gravitational/gravity/pull/809)).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://github.com/kubernetes/kubernetes/issues/83253) for more information.

### 6.0.10 (October 17th, 2019)

#### Improvements

* Add support for discovering upstream DNS servers from systemd-resolved configuration ([#740](https://github.com/gravitational/gravity/pull/740)).
* Improve `gravity report` to capture additional network configuration ([#769](https://github.com/gravitational/gravity/pull/769)).

#### Bugfixes

* Upgrade Kubernetes to `v1.14.08` (CVE-2019-11253) ([#810](https://github.com/gravitational/gravity/pull/810)).
* Fix the issue with join failing with "bad username or password" when using auto-scaling groups on AWS ([#790](https://github.com/gravitational/gravity/pull/790)).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://github.com/kubernetes/kubernetes/issues/83253) for more information.

### 6.2.1 (October 11th, 2019)

#### Improvements

* Add support for discovering upstream DNS servers from systemd-resolved configuration ([#782](https://github.com/gravitational/gravity/pull/782)).
* Improve `gravity report` to capture additional network configuration ([#769](https://github.com/gravitational/gravity/pull/769)).
* Add ability to specify default cloud provider in cluster manifest ([#761](https://github.com/gravitational/gravity/pull/761)).
* Add `ebtable_filter` to the list of required gravity kernel modules ([#724](https://github.com/gravitational/gravity/pull/724)).
* Increase timeout on healthz check and allow optional configuration by environment variable ([#752](https://github.com/gravitational/gravity/pull/752)).

#### Bugfixes

* Fix the issue with join failing with "bad username or password" when using auto-scaling groups on AWS ([#789](https://github.com/gravitational/gravity/pull/789)).
* Fix the issue with web UI installer displaying the login screen ([#793](https://github.com/gravitational/gravity/pull/793)).
* Fix the issue with UI showing "user not found" error after choosing a password for a new user ([#793](https://github.com/gravitational/gravity/pull/793)).
* Fix the issue with `gravity report` accessing journal files ([#732](https://github.com/gravitational/gravity/pull/732)).

### 6.1.7 (October 11th, 2019)

#### Bugfixes

* Fix the issue with join failing with "bad username or password" when using auto-scaling groups on AWS ([#789](https://github.com/gravitational/gravity/pull/789)).
* Fix the issue with web UI installer displaying the login screen ([#793](https://github.com/gravitational/gravity/pull/793)).
* Fix the issue with UI showing "user not found" error after choosing a password for a new user ([#793](https://github.com/gravitational/gravity/pull/793)).

### 6.1.6 (October 10th, 2019)

#### Improvements

* Improves `gravity report` to capture additional network configuration ([#770](https://github.com/gravitational/gravity/pull/770)).
* Adds ability to specify default cloud provider in cluster manifest ([#760](https://github.com/gravitational/gravity/pull/760)).
* Provides additional error information when an operation fails ([#746](https://github.com/gravitational/gravity/pull/746)).
* Increase timeout on healthz check and allow optional configuration by environment variable ([#744](https://github.com/gravitational/gravity/pull/744)).
* Add support for discovering upstream DNS servers from systemd-resolved configuration ([#739](https://github.com/gravitational/gravity/pull/739)).
* Update debian containers to be based off debian buster ([#770](https://github.com/gravitational/gravity/pull/770)).
* Add `ebtable_filter` to the list of required gravity kernel modules ([#725](https://github.com/gravitational/gravity/pull/725)).

#### Bugfixes

* Fixes a race condition within docker libraries used by gravity ([#778](https://github.com/gravitational/gravity/pull/778)).
* Fix an issue with `gravity report` accessing journal files ([#733](https://github.com/gravitational/gravity/pull/733)).

### 6.2.0 (September 24th, 2019)

#### Improvements

* Upgrade Kubernetes to `v1.16.0`.

### 6.1.5 (September 18th, 2019)

#### Bugfixes

* Upgrade Kubernetes to `v1.15.4` (CVE-2019-11251).
* Upgrade Teleport to `3.2.12`.
* Address several issues with cluster stability after etcd upgrades.
* Fix a vulnerability in the decompression of application bundles.

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!msg/kubernetes-announce/YYtEFdFimZ4/nZnOezZuBgAJ) for more information.

!!! warning
    This release fixes a security vulnerability in teleport. Please see
    [Teleport Announcement](https://github.com/gravitational/teleport/releases/tag/v4.0.5) for more information.

### 6.0.9 (September 18th, 2019)

#### Bugfixes

* Upgrade Kubernetes to `v1.14.7` (CVE-2019-11251).
* Upgrade Teleport to `3.2.12`.
* Address several issues with cluster stability after etcd upgrades.
* Fix a vulnerability in the decompression of application bundles.

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!msg/kubernetes-announce/YYtEFdFimZ4/nZnOezZuBgAJ) for more information.

!!! warning
    This release fixes a security vulnerability in teleport. Please see
    [Teleport Announcement](https://github.com/gravitational/teleport/releases/tag/v4.0.5) for more information.

### 6.0.8 (September 11th, 2019)

#### Bugfixes

* Update kubelet configuration to respect `allowPrivileged` flag.

### 6.1.4 (September 10th, 2019)

#### Bugfixes

* Fix `allowPrivileged` flag placement in the manifest schema.

### 6.0.7 (September 10th, 2019)

#### Bugfixes

* Fix `allowPrivileged` flag placement in the manifest schema.

### 6.1.3 (September 9th, 2019)

#### Improvements

* Add ability to run privileged containers. See [Running Privileged Containers](faq.md#running-privileged-containers) for details.

### 6.0.6 (September 9th, 2019)

#### Improvements

* Add ability to run privileged containers. See [Running Privileged Containers](faq.md#running-privileged-containers) for details.

### 6.1.2 (August 26th, 2019)

#### Bugfixes

* Upgrade golang to `v1.12.9` (CVE-2019-9512, CVE-2019-9514)
* Upgrade Kubernetes to `v1.15.2` (CVE-2019-9512, CVE-2019-9514).

!!! warning
    This release fixes a security vulnerability in golang used by gravity and kubernetes. Please see
    [Netflix Announcement](https://github.com/Netflix/security-bulletins/blob/master/advisories/third-party/2019-002.md) for more information.

### 6.0.5 (August 26th, 2019)

#### Bugfixes

* Upgrade golang to `v1.12.9` (CVE-2019-9512, CVE-2019-9514)
* Upgrade Kubernetes to `v1.14.6` (CVE-2019-9512, CVE-2019-9514).

!!! warning
    This release fixes a security vulnerability in golang used by gravity and kubernetes. Please see
    [Netflix Announcement](https://github.com/Netflix/security-bulletins/blob/master/advisories/third-party/2019-002.md) for more information.

### 6.0.4 (August 15th, 2019)

#### Bugfixes

* Fix an issue with connecting to nodes with non-resolvable hostnames via Cluster Control Panel.
* Fix an issue with some audit events not being returned when using custom date picker.

### 6.0.3 (August 14th, 2019)

#### Bugfixes

* Do not set auth gateway public addresses to the cluster name during installation.

### 6.1.1 (August 6th, 2019)

#### Improvements

* Improve reporting of time synchronization issues during join process.
* Improve resiliency of node join process.
* Improve removal of a node where the join process has been aborted.

#### Bugfixes

* Update etcd gateway configuration as masters are removed or added to the cluster.
* Upgrade Kubernetes to `v1.15.2` (CVE-2019-11247, CVE-2019-11249).
* Fix crash in `gravity license show`.
* Fixes a couple issues with initializing the installer service.

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/Vf31dXp0EJc) for more information.

### 6.0.2 (August 6th, 2019)

#### Improvements

* Improve reporting of time synchronization issues during join process.
* Improve resiliency of node join process.
* Improve removal of a node where the join process has been aborted.

#### Bugfixes

* Update etcd gateway configuration as masters are removed or added to the cluster.
* Upgrade Kubernetes to `v1.14.5` (CVE-2019-11247, CVE-2019-11249).
* Fix crash in `gravity license show`.
* Fixes a couple issues with initializing the installer service.

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/Vf31dXp0EJc) for more information.

### 6.1.0 (August 2nd, 2019)

#### Improvements

* Upgrade Kubernetes to `1.15.1`.

### 6.0.1 (July 18th, 2019)

#### Bugfixes

* Skip Helm and Docker login during `tele login` in insecure mode.

### 6.0.0 (July 17th, 2019)

#### Improvements

* Update monitoring application to version `6.0.4`.
* Tweak help messages for `gravity` / `tele` command-line tools and their flags.

#### Bugfixes

* Fix an issue with inaccurate descriptions for some audit log events.
* Fix an issue with audit log events not properly emitted for upgrade operation.
* Fix an issue with not all `helm` commands working from host.
* Fix an issue with install failure if cluster image includes "resources" sub-directory.

### 6.0.0-rc.5

#### Improvements

* Reduce liveness probe delay on `gravity-site`.
* Upgrade Teleport to `3.2.7`.
* Improve resiliency of the `wait` phase.
* Add logs of terminated containers to debug report.
* Various user-interface tweaks.

#### Bugfixes

* Fix an issue with deleting a node via user-interface.

### 6.0.0-rc.4

#### Bugfixes

* Fix a security issue with insecure decompression of application bundles.
* Fix a security issue that allowed remote code execution in the tele cli tool.
* Fix a security issue with missing ACLs in internal API.
* Fix a security issue with install scripts command injection.
* Fix a security issue that allowed for two factor authentication to be bypassed.
* Fix a security issue that allowed for cross-site scripting in Internet Explorer.

!!! warning
    This release fixes security vulnerabilities within Gravity. Please see
    [Gravity Enterprise Announcement](https://gravitational.zendesk.com/hc/en-us/articles/360025697553-Gravity-Enterprise-6-0-0-rc-4-5-6-4-5-5-12-5-2-13-Security-Update) for more information.

### 6.0.0-rc.3

#### Improvements

* Automatically generate debug report when an operation fails.
* Upgrade Kubernetes to `v1.14.2` (CVE-2019-1002101)

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/zqklrWzeA2c) for more information.

### 6.0.0-rc.2

#### Improvements

* Introduce time drift check.
* Update Teleport to `3.2.6`.
* Add RHEL 8 to supported distros.
* Update [Logrange](https://logrange.io) to `0.1.1`.

#### Bugfixes

* Update web UI to properly hide components based on user permissions.
* Update `RuntimeEnvironment` resource to properly support environment variables with commas.

### 6.0.0-rc.1

#### Improvements

* New web user interface for Cluster and Hub.
* Improve resiliency of install operation: dropped SSH session doesn't interrupt it anymore.
* Auto-load required kernel modules and kernel parameters during platform startup.
* Upgrade logging stack to use [Logrange](https://logrange.io/) streaming database.

#### Bugfixes

* Make gravity bypass proxies for local addresses.
* Fix an issue with upgrade operation sometimes failing on the etcd upgrade step.

### 6.0.0-beta.1

#### Improvements

* Upgrade Teleport to `3.2.5`.
* Replace InfluxDB/Kapacitor monitoring stack with Prometheus/Alertmanager.
* Upgrade Docker to `18.09.5`.
* Upgrade Kubernetes to `1.14.1`.
* Add support for using `helm` directly from host.

## 5.x Releases

### 5.5.38 LTS (March 10th, 2020)

#### Improvements

* Add ability to view [cluster status history](cluster.md#cluster-status-history) using `gravity status history` command. ([#1116](https://github.com/gravitational/gravity/pull/1116))
* Add RHEL 8 to the list of supported distros. ([#1144](https://github.com/gravitational/gravity/pull/1144))
* Add client/server version information to `gravity status`. ([#1164](https://github.com/gravitational/gravity/pull/1164))
* Make upgrade agents deployment more tolerant to networking issues. ([#1174](https://github.com/gravitational/gravity/pull/1174))
* Update default Grafana dashboards to include data from medium and long retentions policies. ([monitoring-app#144](https://github.com/gravitational/monitoring-app/pull/144))
* Update built-in Grafana configuration to disable update checks and analytics. ([monitoring-app#146](https://github.com/gravitational/monitoring-app/pull/146))

#### Bugfixes

* Fix an issue with CoreDNS config generation during upgrade. ([#1163](https://github.com/gravitational/gravity/pull/1163))
* Fix an issue with upgrade operation starting even if some agents failed to deploy. ([#1174](https://github.com/gravitational/gravity/pull/1174))
* Fix an issue with Teleport nodes failing to upload recorded sessions after upgrading from older clusters. ([#1216](https://github.com/gravitational/gravity/pull/1216))
* Fix an issue with localhost resolution inside monitoring application containers. ([monitoring-app#145](https://github.com/gravitational/monitoring-app/pull/145))
* Mitigate issues with launching buster-based Docker containers in some environments by rolling back to stretch. ([#1228](https://github.com/gravitational/gravity/pull/1228), [monitoring-app#149](https://github.com/gravitational/monitoring-app/pull/149))

### 5.5.37 LTS (February 11th, 2020)

#### Improvements

* Include etcd dump and gravity status output into gravity report. ([#1081](https://github.com/gravitational/gravity/pull/1081))

#### Bugfixes

* Fix the issue with service name resolution inside init containers in Gravity hooks. ([#1095](https://github.com/gravitational/gravity/pull/1095))

### 5.5.36 LTS (February 6th, 2020)

#### Bugfixes

* Fix an issue with Monitoring tab not working due to InfluxDB authorization failures ([#1076](https://github.com/gravitational/gravity/pull/1076)).

### 5.5.35 LTS (February 4th, 2020)

#### Improvements

* Make username/password for SMTP configuration optional ([#1060](https://github.com/gravitational/gravity/pull/1060))

#### Bugfixes

* Fix an issue with flannel incorrectly recovering from a watch failure ([#1071](https://github.com/gravitational/gravity/pull/1071))
* Fix an issue with merging ClusterConfiguration resource and validation checks ([#1061](https://github.com/gravitational/gravity/pull/1061))


### 5.5.34 LTS (January 28th, 2020)

#### Bugfixes

* Fix the issue with updating pod CIDR via Cluster Configuration resource ([#1040](https://github.com/gravitational/gravity/pull/1040)).

### 5.5.33 LTS (January 24th, 2020)

#### Bugfixes

* Fix the issue with UI showing "user not found" error after choosing a password for a new user ([#1030](https://github.com/gravitational/gravity/pull/1030)).
* Fix the issue with devices being unmounted during system uninstall ([#1045](https://github.com/gravitational/gravity/pull/1045)).

### 5.5.32 LTS (December 23rd, 2019)

#### Bugfixes

* Fix the issue with installing cluster images that require a license ([#990](https://github.com/gravitational/gravity/pull/990)).

### 5.5.31 LTS (December 20th, 2019)

#### Bugfixes

* Fix a security issue where secrets are being reused for multiple certificates and secrets are not being rotated during certificate rotation [#981](https://github.com/gravitational/gravity/pull/981).

!!! warning
    This release fixes a security vulnerability in gravity.


### 5.5.28 LTS (November 4th, 2019)

#### Bugfixes

* Fix the issue with join operation failing if started while installer is still running ([#861](https://github.com/gravitational/gravity/pull/861)).

### 5.5.27 LTS (November 1st, 2019)

#### Improvements

* Preflight checks are now executed during join operation ([#843](https://github.com/gravitational/gravity/pull/843)).
* Update `gravity check` command to support upgrade preflight checks ([#845](https://github.com/gravitational/gravity/pull/845)).
* Disable unused Docker bridge interface ([#851](https://github.com/gravitational/gravity/pull/851),  [planet#517](https://github.com/gravitational/planet/pull/517)).
* Update kernel module checker to support 5.0/5.1 Linux kernels ([#851](https://github.com/gravitational/gravity/pull/851), [planet#523](https://github.com/gravitational/planet/pull/523)).

#### Bugfixes

* Fix the issue with application-only (without runtime) upgrades ([#847](https://github.com/gravitational/gravity/pull/847)).
* Restore `procps` package in the planet container ([#851](https://github.com/gravitational/gravity/pull/851), [planet#525](https://github.com/gravitational/planet/pull/525)).

### 5.5.26 LTS (October 17th, 2019)

#### Bugfixes

* Upgrade Kubernetes to `v1.13.12` (CVE-2019-11253) ([#811](https://github.com/gravitational/gravity/pull/811)).
* Fixes an issue with timeouts while validating agent connections ([#777](https://github.com/gravitational/gravity/pull/777)).
* Fixes an issue where upgrade could fail with `latest package not found` error in gravity-site ([#813](https://github.com/gravitational/gravity/pull/813)).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://github.com/kubernetes/kubernetes/issues/83253) for more information.

### 5.5.24 LTS (October 15th, 2019)

#### Improvements

* Add ability to override peer connect timeout when joining a cluster ([#777](https://github.com/gravitational/gravity/pull/777)).


### 5.2.16 LTS (October 11th, 2019)

#### Improvements

* Improves `gravity report` to capture additional network configuration ([#773](https://github.com/gravitational/gravity/pull/773)).
* Increase timeout on healthz check and allow optional configuration by environment variable ([#737](https://github.com/gravitational/gravity/pull/737)).

#### Bugfixes

* Skip missing mount points when checking filesystem usage ([#786](https://github.com/gravitational/gravity/pull/786)).


### 5.5.23 LTS (October 11th, 2019)

#### Improvements

* Only capture world-readable details in `gravity report` ([#787](https://github.com/gravitational/gravity/pull/787)).

#### Bugfixes

* Fix an issue in 'tele build' not correctly marking intermediate packages ([#775](https://github.com/gravitational/gravity/pull/775)).
* Skip missing mount points when checking filesystem usage ([#779](https://github.com/gravitational/gravity/pull/779)).


### 5.5.22 LTS (October 8th, 2019)

#### Bugfixes

* Fix an issue with monitoring application upgrade from 5.2 ([#136](https://github.com/gravitational/monitoring-app/pull/136)).
* Fix an issue with CoreDNS sometimes missing upstream resolvers ([#742](https://github.com/gravitational/gravity/pull/742)).
* Fix an issue with incorrectly counting nodes when validating license ([#751](https://github.com/gravitational/gravity/pull/751)).
* Fix an issue with system information collector failing to parse `/etc/system-release` if it contained comments ([#731](https://github.com/gravitational/gravity/pull/731)).

### 5.6.8 (September 18th, 2019)

#### Bugfixes

* Upgrade Kubernetes to `v1.14.7` (CVE-2019-11251).
* Upgrade Teleport to `3.0.6-gravity`.
* Address several issues with cluster stability after etcd upgrades.
* Fix a vulnerability in the decompression of application bundles.

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!msg/kubernetes-announce/YYtEFdFimZ4/nZnOezZuBgAJ) for more information.

!!! warning
    This release fixes a security vulnerability in teleport. Please see
    [Teleport Announcement](https://github.com/gravitational/teleport/releases/tag/v4.0.5) for more information.

### 5.5.21 LTS (September 26th, 2019)

#### Improvements

* Add support for direct upgrades of clusters based on Gravity 5.0.x ([#637](https://github.com/gravitational/gravity/pull/637)).
* Update github.com/gravitational/monitoring-app to 5.5.3 ([#644](https://github.com/gravitational/gravity/pull/644)).
  * Disable NodePort access to Kapacitor service.
  * Randomly generate passwords for superuser, telegraf, heapster and grafana users in InfluxDB database during installation and upgrades.

#### Bugfixes

* Fix an issue with `gravity report` not capturing planet journal logs ([#684](https://github.com/gravitational/gravity/pull/684)).
* Fix a package ordering issue in `tele build` ([#712](https://github.com/gravitational/gravity/pull/712)).
* Fix an issue with the time drift checker ([#710](https://github.com/gravitational/gravity/pull/710)).

### 5.2.15 LTS (September 19th, 2019)

#### Improvements

* Update github.com/gravitational/monitoring-app to 5.2.5 ([#642](https://github.com/gravitational/gravity/pull/642)).
* Add support for intermediate upgrades ([#711](https://github.com/gravitational/gravity/pull/711), [#709](https://github.com/gravitational/gravity/pull/709), [#612](https://github.com/gravitational/gravity/pull/612)).

#### Bugfixes

* Improve OS metadata parsing in agents ([#721](https://github.com/gravitational/gravity/pull/721)).

### 5.5.20 LTS (September 18th, 2019)

#### Bugfixes

* Upgrade Kubernetes to `v1.13.11` (CVE-2019-11251).
* Upgrade Teleport to `3.0.6-gravity`.
* Address several issues with cluster stability after etcd upgrades.
* Fix a vulnerability in the decompression of application bundles.

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!msg/kubernetes-announce/YYtEFdFimZ4/nZnOezZuBgAJ) for more information.

!!! warning
    This release fixes a security vulnerability in teleport. Please see
    [Teleport Announcement](https://github.com/gravitational/teleport/releases/tag/v4.0.5) for more information.

### 5.5.19 LTS (September 10th, 2019)

#### Improvements

* Add support for including intermediate runtimes in `tele build`.
* Add Ubuntu 18.04 to the list of supported distros.
* Remove the hard limit of 3 master nodes.

#### Bugfixes

* Wait for `kube-system` namespace to be created during the installation.
* Update `tele push` to treat existing applications and their dependencies gracefully.

### 5.0.35 (September 2nd, 2019)

#### Bugfixes

* Upgrade golang to `v1.11.13` (CVE-2019-9512, CVE-2019-9514)
* Upgrade Kubernetes to `v1.9.13-gravitational` (CVE-2019-9512, CVE-2019-9514).

!!! warning
    Gravitational has backported the fix for CVE-2019-9512 and CVE-2019-9514 to kubernetes version 1.9.13-gravitational.
    This release fixes a security vulnerability in golang used by gravity and kubernetes. Please see
    [Netflix Announcement](https://github.com/Netflix/security-bulletins/blob/master/advisories/third-party/2019-002.md) for more information.

### 5.6.7 (August 26th, 2019)

#### Bugfixes

* Upgrade golang to `v1.12.9` (CVE-2019-9512, CVE-2019-9514)
* Upgrade Kubernetes to `v1.14.6` (CVE-2019-9512, CVE-2019-9514).

!!! warning
    This release fixes a security vulnerability in golang used by gravity and kubernetes. Please see
    [Netflix Announcement](https://github.com/Netflix/security-bulletins/blob/master/advisories/third-party/2019-002.md) for more information.

### 5.5.18 LTS (August 28th, 2019)

#### Bugfixes

* Fix installer tarball to include the correct gravity binary.

### 5.5.17 LTS (August 26th, 2019)

#### Bugfixes

* Upgrade golang to `v1.12.9` (CVE-2019-9512, CVE-2019-9514)
* Upgrade Kubernetes to `v1.13.10` (CVE-2019-9512, CVE-2019-9514).

!!! warning
    This release fixes a security vulnerability in golang used by gravity and kubernetes. Please see
    [Netflix Announcement](https://github.com/Netflix/security-bulletins/blob/master/advisories/third-party/2019-002.md) for more information.

### 5.6.6 (August 6th, 2019)

#### Bugfixes

* Update etcd gateway configuration as masters are removed or added to the cluster.
* Upgrade Kubernetes to `v1.14.5` (CVE-2019-11247, CVE-2019-11249).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/Vf31dXp0EJc) for more information.

### 5.5.15 LTS (August 6th, 2019)

#### Bugfixes

* Update etcd gateway configuration as masters are removed or added to the cluster.
* Upgrade Kubernetes to `v1.13.9` (CVE-2019-11247, CVE-2019-11249).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/Vf31dXp0EJc) for more information.

### 5.2.14 LTS (July 30th, 2019)

#### Improvements

* Remove dependency on system user/group being present in local `/etc/passwd` and `/etc/group` databases.
* Generate credentials for InfluxDB, Telegraf and Grafana during installation and update.

### 5.5.14 LTS (July 24th, 2019)

#### Bugfixes

* Fix an issue with managing monitoring resources via `gravity resource` command.

### 5.6.5 (July 18th, 2019)

#### Bugfixes

* Workaround for installation failures when populating the docker registry.

### 5.5.13 LTS (July 18th, 2019)

#### Bugfixes

* Workaround for installation failures when populating the docker registry.
* Fix an issue with applications that contain a resources subfolder failing to install.

#### Improvements

* Installations that previously used a dedicated devicemapper volume will now be reformatted and reused after upgrade.

### 5.6.4

#### Bugfixes

* Fix a security issue with insecure decompression of application bundles.
* Fix a security issue that allowed remote code execution in the tele cli tool.
* Fix a security issue with missing ACLs in internal API.
* Fix a security issue with install scripts command injection.
* Fix a security issue that allowed for two factor authentication to be bypassed.
* Fix a security issue that allowed for cross-site scripting in Internet Explorer.

!!! warning
    This release fixes security vulnerabilities within Gravity. Please see
    [Gravity Enterprise Announcement](https://gravitational.zendesk.com/hc/en-us/articles/360025697553-Gravity-Enterprise-6-0-0-rc-4-5-6-4-5-5-12-5-2-13-Security-Update) for more information.

### 5.5.12

#### Bugfixes

* Fix a security issue with insecure decompression of application bundles.
* Fix a security issue that allowed remote code execution in the tele cli tool.
* Fix a security issue with missing ACLs in internal API.
* Fix a security issue with install scripts command injection.
* Fix a security issue that allowed for two factor authentication to be bypassed.
* Fix a security issue that allowed for cross-site scripting in Internet Explorer.

!!! warning
    This release fixes security vulnerabilities within Gravity. Please see
    [Gravity Enterprise Announcement](https://gravitational.zendesk.com/hc/en-us/articles/360025697553-Gravity-Enterprise-6-0-0-rc-4-5-6-4-5-5-12-5-2-13-Security-Update) for more information.

### 5.2.13

#### Bugfixes

* Fix a security issue with insecure decompression of application bundles.
* Fix a security issue that allowed remote code execution in the tele cli tool.
* Fix a security issue with missing ACLs in internal API.
* Fix a security issue with install scripts command injection.
* Fix a security issue that allowed for two factor authentication to be bypassed.
* Fix a security issue that allowed for cross-site scripting in Internet Explorer.

!!! warning
    This release fixes security vulnerabilities within Gravity. Please see
    [Gravity Enterprise Announcement](https://gravitational.zendesk.com/hc/en-us/articles/360025697553-Gravity-Enterprise-6-0-0-rc-4-5-6-4-5-5-12-5-2-13-Security-Update) for more information.

### 5.6.3

#### Improvements

* Upgrade Kubernetes to `v1.14.2` (CVE-2019-1002101)

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/zqklrWzeA2c) for more information.

### 5.5.11

#### Improvements

* Upgrade Kubernetes to `v1.13.6` (CVE-2019-1002101)

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/zqklrWzeA2c) for more information.

### 5.6.2

#### Improvements

* Add support for using `helm` directly from host.

### 5.5.10 LTS

#### Bugfixes

* Fix an issue with automatic NO_PROXY rules that break some cluster operations.
* Fix an issue with updating or removing environment configuration.

### 5.5.9 LTS

#### Improvements

* Environment variables now support quoted values.
* Automatic creation of NO_PROXY rules for internal cluster communications.
* Improved validation of gravity-site upgrade.
* Wormhole CNI plugin interfaces will now be removed when gravity is uninstalled.
* The cluster-admin role bindings for default and kube-system namespaces have been separated.

#### Bugfixes

* Fixed KUBE_APISERVER_FLAGS environment variable on kube-apiserver unit.

### 5.5.8 LTS

#### Improvements

* Add support for using `helm` directly from host.

### 5.6.1

#### Improvements

* Upgrade Docker to `18.09.5`.

### 5.5.7 LTS

#### Bugfixes

* Fix an issue with completed status hook pods not being cleaned up.

### 5.5.6 LTS

#### Bugfixes

* Fix an issue with adjusting user volume permissions during upgrade.

### 5.6.0

#### Improvements

* Upgrade Kubernetes to `v1.14.0`.

### 5.0.33 LTS

#### Bugfixes

* Fix a regression with `gravity upgrade --complete`

### 5.0.32 LTS

#### Bugfixes

* Fix an issue with upgrades for applications that were packaged with multiple versions of planet.

### 5.5.5 LTS

#### Improvements

* Upgrade helm and tiller to `v2.12.3`.

### 5.5.4 LTS

#### Improvements

* Upgrade CNI plugins to 0.7.5 (CVE-2019-9946).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/announce-security-release-of-kubernetes-affecting-certain-network-configurations-with-cni-releases-1-11-9-1-12-7-1-13-5-and-1-14-0-cve-2019-9946/5713) for more information.

### 5.5.3 LTS

#### Improvements

* Upgrade Kubernetes to `v1.13.5` (CVE-2019-1002101).

#### Bugfixes

* Fix an issue with CoreDNS crash when local nameserver is present in host's `/etc/resolv.conf`.

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/announce-security-release-of-kubernetes-kubectl-potential-directory-traversal-releases-1-11-9-1-12-7-1-13-5-and-1-14-0-cve-2019-1002101/5712) for more information.


### 5.4.10

#### Improvements

* Upgrade Kubernetes to `v1.13.5` (CVE-2019-1002101).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/announce-security-release-of-kubernetes-kubectl-potential-directory-traversal-releases-1-11-9-1-12-7-1-13-5-and-1-14-0-cve-2019-1002101/5712) for more information.

### 5.2.12 LTS

#### Improvements

* Upgrade Kubernetes to `v1.11.9` (CVE-2019-1002101).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/announce-security-release-of-kubernetes-kubectl-potential-directory-traversal-releases-1-11-9-1-12-7-1-13-5-and-1-14-0-cve-2019-1002101/5712) for more information.

### 5.0.30 LTS

#### Improvements

* Improve resiliency of the election install phase.

### 5.5.2 LTS

#### Bugfixes

* Fix an issue with `tele` not recognizing some resources.
* Fix an issue with creating `smtp` and `alert` resources in cluster.
* Fix an issue with using custom state directory during `tele build`.
* Fix an issue with new packages not being deleted when performing a rollback.

### 5.5.1 LTS

#### Improvements

* Improve shrink operation behavior when using Auto-Scaling Groups on AWS.

#### Bugfixes

* Fix an issue with `gravity report` sometimes producing unreadable tarball.

### 5.4.9

#### Improvements

* Improve shrink operation behavior when using Auto-Scaling Groups on AWS.

### 5.5.0 LTS

#### Improvements

* Multiple UX tweaks for `gravity app list` command.
* Better validation for the cloud configuration resource.

#### Bugfixes

* Fix an issue with `kubectl` not working on host when using custom state directory.
* Fix an issue with `gravity status` always displaying degraded status on regular nodes.
* Fix an issue with shrink operation sometimes spawning on the leaving node.

### 5.2.11 LTS

#### Bugfixes

* Fix an issue with manually completing rolled back upgrade plan.

### 5.3.9

#### Improvements

* Use `overlay2` as default storage driver.
* Enable aggregation layer on the Kubernetes API server.

#### Bugfixes

* Fix an issue with manually completing rolled back upgrade plan.

### 5.5.0-rc.1

#### Improvements

* Introduce `ClusterConfiguration` resource, see [Configuring Cluster](config.md#general-cluster-configuration) for details.
* Introduce `RuntimeEnvironment` resource, see [Configuring Runtime Environment Variables](config.md#runtime-environment-variables) for details.
* Update 'gravity plan' to support all cluster operations.

### 5.5.0-beta.2

#### Bugfixes

* Update to Kubernetes 1.13.4 (CVE-2019-1002100).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/kubernetes-security-announcement-v1-11-8-1-12-6-1-13-4-released-to-address-medium-severity-cve-2019-1002100/5147) for more information.

### 5.4.7

#### Bugfixes

* Update to Kubernetes 1.13.4 (CVE-2019-1002100).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/kubernetes-security-announcement-v1-11-8-1-12-6-1-13-4-released-to-address-medium-severity-cve-2019-1002100/5147) for more information.

### 5.2.10 LTS

#### Bugfixes

* Update to Kubernetes 1.13.4 (CVE-2019-1002100).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/kubernetes-security-announcement-v1-11-8-1-12-6-1-13-4-released-to-address-medium-severity-cve-2019-1002100/5147) for more information.

### 5.5.0-beta.1

#### Improvements

* Upgrade Kubernetes to `v1.13.3`.
* Default to `tele` version when selecting base image during `tele build`.
* Improve Teleport nodes resiliency.

#### Bugfixes

* Fix an issue with `tele login` when Docker/Helm are not available.
* Fix an issue with Teleport nodes missing some labels after upgrade.

### 5.5.0-alpha.9

#### Improvements

* UX improvements to `tele ls` command.

#### Bugfixes

* Fix an issue with pulling base cluster image with `tele pull`.

### 5.5.0-alpha.8

#### Improvements

* Introduce `AuthGateway` resource. See [Configuring Authentication Gateway](config.md#cluster-authentication-gateway)
for details.
* UX improvements to `tele` CLI.

#### Bugfixes

* Update to Docker 18.06.2 (cve-2019-5736).
* Update gravity-site healthz endpoint to more reliably indicate failures.
* Use xdg-open to launch default browser.
* Fix an issue with `tele logout` when no helm and docker executable are present.

!!! warning
    This release fixes a security vulnerability in runc. Please see
    [Kubnernetes Blog](https://kubernetes.io/blog/2019/02/11/runc-and-cve-2019-5736/) for more information.

### 5.4.6

#### Bugfixes

* Update to Docker 18.06.2 (cve-2019-5736).

!!! warning
    This release fixes a security vulnerability in runc. Please see
    [Kubnernetes Blog](https://kubernetes.io/blog/2019/02/11/runc-and-cve-2019-5736/) for more information.

### 5.2.9 LTS

#### Bugfixes

* Fix the issue with "gravity gc" failing to collect packages on regular nodes.


### 5.2.8 LTS

#### Bugfixes

* Update docker-runc to avoid security vulnerability (cve-2019-5736).

!!! warning
    This release fixes a security vulnerability in runc. Please see
    [Kubnernetes Blog](https://kubernetes.io/blog/2019/02/11/runc-and-cve-2019-5736/) for more information.

### 5.0.29

#### Bugfixes

* Update docker-runc to avoid security vulnerability (cve-2019-5736).
* Update xterm.js to avoid security vulnerability (CVE-2019-0542).
* Restrict Teleport cipher suites.

!!! warning
    This release fixes a security vulnerability in runc. Please see
    [Kubnernetes Blog](https://kubernetes.io/blog/2019/02/11/runc-and-cve-2019-5736/) for more information.

### 5.2.7 LTS

#### Bugfixes

* Update teleport binaries to match embedded version.
* Update gravity-site healthz endpoint to more reliably indicate failures.

### 5.5.0-alpha.7

#### Improvements

* Restrict Teleport cipher suites.
* Use `overlay2` storage driver by default.
* Add Helm chart repository and Docker registry support to clusters.
* Update Teleport to `v3.0.4`.
* Enable Teleport's Kubernetes proxy integration.
* Multiple installer UX enhancements.
* Add ability to exclude certain applications from installation. See [Excluding System Applications](pack.md#system-extensions) for details.

#### Bugfixes

* Update `gravity leave` command to clean up CNI interface.
* Fix an issue with vendoring unrecognized resources.
* Fix a potential connection leak.
* Fix a potential panic in `gravity join` command.

### 5.4.5

#### Bugfixes

* Backport support for builtin kernel modules in preflight checks.

### 5.4.4

#### Bugfixes

* Adjust system resources limits for CoreDNS.
* Fix an issue with propagation of kubectl command line options from wrapper script to kubectl.
* Set overlay2 as default docker graph driver.

### 5.4.3

#### Bugfixes

* Fixes a connection leak when fetching agent reports.
* Don't copy rotate option to resolv.conf used inside planet container.
* Updates kubernetes to 1.13.2.
* Updates etcd to 3.3.11

!!! warning
    Kubernetes 1.13.2 and etcd 3.3.11 fix a denial of service vulnerability. Please see
    [National Vulnerability Database](https://nvd.nist.gov/vuln/detail/CVE-2018-16875) for more information.

### 5.5.0-alpha.6

#### Improvements

* Adjust system resources limits for CoreDNS.

### 5.5.0-alpha.5

#### Bugfixes

* Set advertise-address on kube-apiserver to fix binding on hosts with multiple network addresses.

### 5.5.0-alpha.4

#### Bugfixes

* Revendor teleport to include security fix.

!!! warning
    Teleport 3.0.3 includes fixes for a security vulnerability. Please see
    [Teleport Announcements](https://gravitational.zendesk.com/hc/en-us/articles/360015185614-Teleport-3-1-2-3-0-3-2-7-7-2-6-10) for more information.

### 5.4.2

#### Bugfixes

* Revendor teleport to 2.4.10.

!!! warning
    Teleport 2.4.10 includes fixes for a security vulnerability. Please see
    [Teleport Announcements](https://gravitational.zendesk.com/hc/en-us/articles/360015185614-Teleport-3-1-2-3-0-3-2-7-7-2-6-10) for more information.

### 5.3.8

#### Improvements

* Add support for creating Gravity resources during install.

### 5.3.7

#### Improvements

* New resource type `runtimeenvironment`. See [Configuring Runtime Environment Variables](config.md#runtime-environment-variables) for details.

### 5.3.6

#### Bugfixes

* Revendor teleport to 2.4.10.

!!! warning
    Teleport 2.4.10 includes fixes for a security vulnerability. Please see
    [Teleport Announcements](https://gravitational.zendesk.com/hc/en-us/articles/360015185614-Teleport-3-1-2-3-0-3-2-7-7-2-6-10) for more information.

### 5.2.6 LTS

#### Bugfixes

* Fix an issue with cluster expansion when applying taints via app.yaml.

### 5.2.5 LTS

#### Bugfixes

* Revendor teleport to 2.4.10.

!!! warning
    Teleport 2.4.10 includes fixes for a security vulnerability. Please see
    [Teleport Announcements](https://gravitational.zendesk.com/hc/en-us/articles/360015185614-Teleport-3-1-2-3-0-3-2-7-7-2-6-10) for more information.

### 5.1.3

#### Bugfixes

* Revendor teleport to 2.4.10.

!!! warning
    Teleport 2.4.10 includes fixes for a security vulnerability. Please see
    [Teleport Announcements](https://gravitational.zendesk.com/hc/en-us/articles/360015185614-Teleport-3-1-2-3-0-3-2-7-7-2-6-10) for more information.

### 5.0.28

#### Bugfixes

* Revendor teleport to 2.4.10.

!!! warning
    Teleport 2.4.10 includes fixes for a security vulnerability. Please see
    [Teleport Announcements](https://gravitational.zendesk.com/hc/en-us/articles/360015185614-Teleport-3-1-2-3-0-3-2-7-7-2-6-10) for more information.

### 5.4.1

#### Bugfixes

* Fix an issue with certain node labels preventing successful installation.

### 5.4.0

#### Improvements

* Upgrade to Kubernetes `v1.13.0`.

### 5.0.27 LTS

#### Bugfixes

* Fix an issue with upgrade failure on clusters with non-master nodes.

### 5.5.0-alpha.3

#### Improvements

* Add support for Wireguard encrypted overlay network. See [Wireguard Encrypted Networking](cluster.md#wireguard-encrypted-networking) for details.
* Reduce writes to AWS SSM store when using AWS integrations.
* Update tiller to 2.11.0
* Add initial support for application catalog. See [Application Catalog](catalog.md) for details.
* Update embedded teleport to 3.0.1

### 5.0.26 LTS

#### Bugfixes

* Fix an issue with upgrade agents failing to start in some cases.

### 5.3.5

#### Bugfixes

* Update Kubernetes to version 1.12.3.

!!! warning
    Kubernetes 1.12.3 includes fixes for CVE-2018-1002105. Please see
    [Issue 71411](https://github.com/kubernetes/kubernetes/issues/71411) for more information.

### 5.2.4 LTS

#### Bugfixes

* Update Kubernetes to version 1.11.5.

!!! warning
    Kubernetes 1.11.5 includes fixes for CVE-2018-1002105. Please see
    [Issue 71411](https://github.com/kubernetes/kubernetes/issues/71411) for more information.

### 5.1.2

#### Bugfixes

* Update Kubernetes to version 1.9.12-gravitational.

!!! warning
    Gravitational has backported the fix for CVE-2018-1002105 to kubernetes version 1.9.12-gravitational. Please see
    [Issue 71411](https://github.com/kubernetes/kubernetes/issues/71411) for more information.

### 5.0.25

#### Bugfixes

* Update Kubernetes to version 1.9.12-gravitational.

!!! warning
    Gravitational has backported the fix for CVE-2018-1002105 to kubernetes version 1.9.12-gravitational. Please see
    [Issue 71411](https://github.com/kubernetes/kubernetes/issues/71411) for more information.

### 5.3.4

#### Improvements

* Update docker to 18.06.1.
* Remove support for docker devicemapper storage driver.

!!! warning
    The docker devicemapper storage driver has been removed from docker 18.06.1. Applications using the devicemapper
    storage driver must be updated to use overlay2.

### 5.3.3

#### Improvements

* Add support for recursive volume mounts. See [Sample Manifest](pack.md#image-manifest) for details.
* Adjust CoreDNS permissions for cluster conformance.

#### Bugfixes

* Fix an issue with `tele build` ignoring `--repository` flag when `--state-dir` flag is provided.
* Fix an issue with installer complaining about "missing DNS config" in some cases.

### 5.2.3 LTS

#### Improvements

* Add support for recursive volume mounts. See [Image Manifest](pack.md#image-manifest) for details.
* Disable `DenyEscalatingExec` admission controller to meet conformance.

### 5.3.2

#### Bugfixes

* Fix an issue with cluster expansion when applying taints via app.yaml.
* Fix an issue with labeling of packages, which could prevent upgrades from completing.

### Improvements

* Improved error message when RPC agent fails to connect.

### 5.2.2 LTS

#### Bugfixes

* Strip original registry when tagging images to local registry when using Helm charts.

### 5.3.1

#### Bugfixes

* Strip original registry when tagging images to local registry when using Helm charts.

### 5.3.0

#### Improvements

* Upgrade to Kubernetes `1.12.1`.
* Replace `kube-dns` with CoreDNS.
* Remove dependency on system user/group being present in local `/etc/passwd` and `/etc/group` databases.

### 5.2.1 LTS

#### Bugfixes

* Fix an issue with open-source `tele` requiring AWS credentials.

### 5.2.0 LTS

#### Improvements

* Make leader election install/upgrade phase more resilient.

#### Bugfixes

* Fix an issue with upgrade recovery scenario depending on etcd.
* Fix an issue with multiple shrink operations being launched sometimes.
* Fix an issue with following operation logs using `gravity status` command.

### 5.2.0-rc.3

#### Bugfixes

* Fix an issue with deploying AWS clusters using provisioner.
* Fix an issue with installers downloaded from the distribution portal.
* Fix an issue with expanding clusters installed via an Ops Center.

### 5.2.0-rc.2

#### Bugfixes

* Fix `tele build` failure when encountering unrecognized resources.

### 5.1.1

#### Improvements

* Speed up image vendoring during tele build.
* Add cleanup phase to the upgrade operation.
* Make new application upload more efficient.

#### Bugfixes

* Fix tele build failure when encountering unrecognized resources.

### 5.2.0-rc.1

#### Improvements

* Introduce gravity terraform provider. See [Terraform Provider](terraform.md) for details.
* Refactor join operation to use FSM approach.
* Suppress selection prompt in UI install flow if the installer machine has a single network interface.
* Improve upgrade operation logging and move default log location to `/var/log`.
* Add a high disk usage check to the cluster health checker.
* Extend cluster health checker with detection of Kubernetes `NotReady` nodes.
* Add support for switching Docker storage drivers during upgrade.
* General improvements to `gravity` and `tele` CLI experience.

#### Bugfixes

* Fail quickly on initial RPC connect error during join.
* Fix the issue with Monitoring tab not loading Grafana interface.

### 5.2.0-alpha.3

#### Security

* Disable exposed tiller service which allows privilege escalation.
* Use mTLS to protect metrics and status endpoints.

#### Improvements

* Deprecate expanding cluster on AWS through UI.
* Add support for overriding DNS listen address during install.

### 5.0.24 LTS

#### Security

* Disable exposed tiller service which allows privilege escalation.

#### Improvements

* Feature to allow deletion and update of continuous influxdb queries.

### 5.0.23 LTS

#### Improvements

* Automatically load kernel modules and set kernel parameters required for
installation. See [Verifying Node Requirements](requirements.md#kernel-modules)
for more info.

#### Bugfixes

* Fix an issue with updating Kapacitor SMTP configuration.

### 5.0.22 LTS

#### Bugfixes

* Fix an issue with prechecks ignoring UDP ports.
* Fix an issue with RPC agent credentials which could prevent upgrade from starting.

### 5.0.21 LTS

#### Bugfixes

* Fix an issue with prechecks failing in certain cases when run multiple times quickly.

### 5.0.20 LTS

#### Improvements

* Add ability to set custom overlay network port by supplying `--vxlan-port` flag to `gravity install` command.

### 5.0.18 LTS

#### Bugfixes

* Fix an issue with preflight checks being run twice in some install scenarios.

### 5.0.17 LTS

#### Bugfixes

* Fix an issue with an install operation in inconsistent state after a failed preflight check.

### 5.0.16 LTS

#### Bugfixes

* Fix Docker registry format incompatibility issue.

### 5.0.15 LTS

#### Bugfixes

* Fix an issue with parsing InfluxDB rollup definitions.

#### Improvements

* Include all possible Kubernetes service hostnames in the cluster certificates.

### 5.0.14 LTS

#### Bugfixes

* Fix an issue with `tele build` not detecting and vendoring all kubernetes resources
* Fix an issue with grafana UI showing a sidebar

### 5.0.13 LTS

#### Improvements

* Add support for SAML authentication connector. See [Configuring SAML Connector](config.md#configuring-saml-connector)
for information on how to configure authentication and authorization via a SAML
provider.

### 5.1.0

#### Improvements

* Update `kube-dns` application to version 1.14.10.
* Preflight checks are executed on expand.

#### Bugfixes

* Fix OS distribution detection for RedHat when lsb_release is installed.
* Fix an issue when configured cloud provider was not propagated to agents.

### 5.0.12 LTS

#### Bugfixes

* Fix a panic when removing an offline node.
* Fix an issue with update inadvertently rotating agent RPC credentials.
* Fix an issue with creating retention policies for InfluxDB.

### 5.0.11 LTS

#### Improvements

* Uploading upgrades to a cluster now uses less disk space.
* Logs for planet services will now automatically be cleared during an upgrade.

#### Bugfixes

* Update flannel to include latest upstream fixes.

### 5.0.10 LTS

#### Improvements

* Add ability to connect arbitrary clusters to an Ops Center.
* Update Grafana to 5.1.0 release.

### 5.0.9 LTS

#### Bugfixes

* Fix an issue where a missing service user could prevent upgrades or cluster expansion.

### 5.0.8 LTS

#### Improvements

* Installation now checks for additional required kernel modules.

#### Bugfixes

* Fix an integration issue with AWS that prevented increasing the size of a cluster through auto scaling.
* Update kube-dns to latest release.
* Fix an issue when configured cloud provider was not propagated to agents.

### 5.0.7 LTS

#### Bugfixes

* Use TLS1.2 with modern cipher suites on planet RPC port.
* InfluxDB, Grafana, Heapster are now only accessible from within the cluster.

### 5.0.6 LTS

#### Bugfixes

* Fix an issue with cluster certificates losing Ops Center SNI host after
upgrade from older versions.
* Fix an issue with Ops Center UI showing clusters a user doesn't have access to.

### 5.1.0-alpha.7

#### Bugfixes

* Fix translation of custom planet images to gravity packages when image reference
is using domain/path components.

### 5.1.0-alpha.6

#### Improvements

* Add `skipIfMissing` for describing optional mounts.
* Add ability to define custom preflight checks.

### 5.1.0-alpha.5

#### Improvements

* Add ability to mount host devices into the Gravity container. See
[Image Manifest](pack.md#image-manifest) for more details.

### 5.1.0-alpha.4

#### Improvements

* Introduce ability to use user-defined base images. See [User-Defined Base Image](pack.md#image-manifest)
for details.

### 5.2.0-alpha.1

#### Improvements

* Add `--dns-zone` flag to `gravity install` command to allow overriding upstreams
for specific DNS zones within the cluster. See flag description in the
[Installation](installation.md#cli-installation) section for details.

### 5.0.4 LTS

#### Bugfixes

* Fix an issue with `--force` flag not propagating correctly when resuming
install/upgrade.

### 5.0.3 LTS

#### Bugfixes

* Exclude Docker device test from upgrade preflight checks.
* Fix an issue with `kubectl` not working from host.

### 5.1.0-alpha.3

#### Improvements

* Add ability to override node tags on GCE during installation.

### 5.1.0-alpha.2

#### Improvements

* Add multizone support for GCE clusters.
* Update preflight checks to check iptables modules. See [requirements](requirements.md#iptables-modules)
for details.
* Add timeout to preflight checks on remote nodes.

#### Bugfixes

* Fix an issue with using custom `--state-dir` when installing on more than a single node.

### 5.0.2 LTS

#### Bugfixes

* Monitoring: fix an RBAC permission issue with collecting metrics from heapster.

### 5.0.1 LTS

#### Bugfixes

* Fix an issue with using custom `--state-dir` when installing on more than a single node.

### 5.1.0-alpha.1

#### Improvements

* Add support for GCE cloud provider. See [Installing on Google Compute Engine](installation.md#google-compute-engine)
for details.

#### Bugfixes

* Fix an issue with `--force` flag not propagating correctly when resuming
install/upgrade.
* Add `NoExecute` taint toleration to hooks.

### 5.0.0 LTS

#### Bugfixes

* Shutdown upgrade agents after upgrade has finished.

### 5.0.0-rc.2

#### Improvements

* Add ability to resume install/update after failure. Check [Troubleshooting Automatic Upgrades](cluster.md#troubleshooting-automatic-upgrades) for details.
* Improve error reporting during install and when viewing operation plan.

#### Bugfixes

* Fix an issue with duplicates in pre-checks failure list.
* Fix an issue with user invite/reset CLI commands displaying incorrect URLs.
* Fix an issue with duplicate progress entries during install.
* Fix a few issues with communication between gravity agents and installer/cluster.

### 5.0.0-rc.1

#### Improvements

* Do not try to fetch cloud metadata in onprem installs.
* Improve handling of agent disconnects.
* When updating node labels, account for possible conflicts.
* Do not display server version for unauthorized users.

#### Bugfixes

* Fix an issue with upgrading clusters that have joined nodes.
* Enable leader election on all nodes after install.
* Fix an issue with creating system user/group on Ubuntu Core.
* Fix support for custom state directory.

### 5.0.0-beta.1

#### Improvements

* Remove node from serf cluster when scaling down.

#### Bugfixes

* Fix an issue with `gravity status` interfering with installation.
* Fix an issue with ignored confirmation response in `gravity leave --force`.
* Fix an issue with hook logs not visible when installing via Ops Center.

### 5.0.0-alpha.17

#### Improvements

* Show application hook output in operation logs for easier troubleshooting.

#### Bugfixes

* Fix an issue with Mac binaries.

### 5.0.0-alpha.16

!!! note
    Due to some build issues this release does not include Mac binaries, please
    use version `5.0.0-alpha.17` or greater if you need to use `tele` for Mac.

#### Improvements

* Add preflight checks for install and upgrade.

#### Bugfixes

* Fix an issue with dns-app upgrade introduced in `5.0.0-alpha.15`.

### 5.0.0-alpha.15

#### Improvements

* Upgrade Kubernetes to `v1.9.6`.
* Add support for more InfluxDB aggregate functions for use in [rollups](https://gravitational.co/gravity/docs/ver/5.x/monitoring/#rollups).

### 5.0.0-alpha.14

#### Improvements

* Standalone installer now supports installing AWS clusters in CLI mode. See
[AWS Installer](installation.md#aws) for more info.

### 5.0.0-alpha.13

#### Bugfixes

* Update Kubernetes to version 1.8.10.
* Ability to override the service user when installing. Read more [here](pack.md#service-user).

#### Bugfixes

* Fix an issue with enabling remote support after remote support has been disabled.

### 5.0.0-alpha.12

#### Improvements

* Increase lifetime of CA certificates used internally within the cluster.
* Add support for separating the endpoint for cluster and user traffic, see [Configuring Ops Center Endpoints](hub.md#post-provisioning) for details.
* Add support for using flags with ./install script.

#### Bugfixes

* Prevent crash when loading an invalid certificate or key pair.
* Update Kubernetes to version 1.8.9.

!!! warning
    Kubernetes 1.8.9 includes fixes for CVE-2017-1002101 and CVE-2017-1002102. Please see
    [Issue 60813](https://github.com/kubernetes/kubernetes/issues/60813) and [Issue 60814](https://github.com/kubernetes/kubernetes/issues/60814)
    for more information.


### 5.0.0-alpha.11

#### Improvements

* Add command to force renewal of cluster certificates.

### 5.0.0-alpha.10

#### Improvements

* Add support for Helm charts. See [Helm Integration](pack.md#helm-integration)
for details.
* Introduce `gravity users add` and `gravity users reset` commands that allow
to invite users and reset user passwords from CLI.
* Overhaul the install procedure to become plan-based.
* Upgrade Teleport to `v2.4.1`.
* Upgrade kube-dns to `1.14.8` and bump its memory limit to account for bursts.

#### Bugfixes

* Correctly propagate `--force` flag when running `gravity leave`.
* Fix an issue with removing nodes that run etcd in proxy mode.

### 5.0.0-alpha.9

#### Bugfixes

* Fixed an issue with upgrading an existing cluster on AWS

### 5.0.0-alpha.8

#### Improvements

* Added node label gravitational.io/advertise-ip
* Updated node label kubernetes.io/hostname to match system hostname on AWS

#### Bugfixes

* Fixed automatic detection of cloud provider on AWS

### 5.0.0-alpha.7

#### Improvements

* Add '-o' flag to tele
* Add support for vendoring CronJob resources
* Improve gravity report to collect systemctl, dmesg, and memory status
* Update Kubernetes to version 1.8.5.

### 5.0.0-alpha.6

#### Improvements

* Improve resiliency of removing a cluster from an Ops Center.

#### Bugfixes

* Fix an integration issue with AWS API that was causing keys validation to fail.

### 5.0.0-alpha.5

#### Improvements

* Add support for trusted clusters, see [Configuring Trusted Clusters](config.md#trusted-clusters-enterprise) for details.
* Improve application install resiliency by retrying on transient errors.
* Improve resiliency when checking for hooks status.

### 5.0.0-alpha.4

#### Improvements

* Add support for configuring docker bridge in promiscuous mode.

### 5.0.0-alpha.3

#### Bugfixes

* Fix problem with deleting clusters created via CLI from the UI.

### 5.0.0-alpha.2

#### Improvements

* Add support for AWS autoscaling groups via [provisioner](https://github.com/gravitational/provisioner#aws-auto-scale-groups-support).
* Add support for explicit [node roles](cluster.md#node-roles).
* Add support for [custom taints](cluster.md#custom-taints).

### 5.0.0-alpha.1

#### Improvements

* Switch to new versioning scheme (includes prereleases metadata)
* Update documentation on kernel plugins

## 4.x Releases

### Instructions on upgrading to 4.23.0+

Upgrading clusters to Gravity 4.23.0 works via the command line interface (CLI) only.
To upgrade a cluster with an application packaged with the Gravity 4.23+
follow the procedure below.

First, the application must be published into the Ops Center. This allows
all connected clusters to see that a new version is available.

On a cluster side, download the new update by logging into one of the cluster nodes and
executing the command below:

```bash
$ gravity update download
```

The next step is to download the latest version of the `gravity` binary.
For example, if upgrading to 4.26.0:

```bash
$ curl https://get.gravitational.io/telekube/bin/4.26.0/linux/x86_64/gravity -o /tmp/gravity
$ chmod +x /tmp/gravity
```

Finally, launch the update process:

```bash
$ /tmp/gravity upgrade
```

This will upgrade the cluster and the system instance of the `gravity` binary,
so the temporary copy in `/tmp` can be discarded.

### 4.68.0 LTS

#### Bugfixes

* Fix an issue with update preflight checks which could prevent upgrades from 3.x.

### 4.67.0 LTS

#### Bugfixes

* Disable docker volume tests during upgrades which could prevent upgrades from completing.

### 4.64.0 LTS

#### Bugfixes

* Update Kubernetes to version 1.7.18-gravitational.

!!! warning
    Gravitational has backported the fix for CVE-2018-1002105 to kubernetes version 1.7.18-gravitational. Please see
    [Issue 71411](https://github.com/kubernetes/kubernetes/issues/71411) for more information.

### 4.63.0 LTS

#### Bugfixes

* Remove filtering of regions based on available AMIs.

### 4.62.0 LTS

#### Bugfixes

* Fix an issue in monitoring when keywords used as field names would cause syntax errors.
* Fix an issue with "gravity status" creating a directory with elevated permissions if run as "root"
  preventing the installation.

### 4.61.0 LTS

#### Bugfixes

* Update Kubernetes to version 1.7.15.
* Fix an issue which could cause the d_type health check to fail.
* Avoid overwriting filesystem metadata if a formatted system state device is specified.

### 4.60.0 LTS

#### Bugfixes

* Update Kubernetes to version 1.7.14.

!!! warning
    Kubernetes 1.7.14 includes fixes for CVE-2017-1002101 and CVE-2017-1002102. Please see
    [Issue 60813](https://github.com/kubernetes/kubernetes/issues/60813) and [Issue 60814](https://github.com/kubernetes/kubernetes/issues/60814)
    for more information.

### 4.59.0 LTS

#### Bugfixes

* Fix a regression that leads to installer failing OS precheck when application
does not specify OS requirements.

### 4.58.0 LTS

#### Bugfixes

* Fix an issue with an invalid default service user configuration when installing from an Ops Center.
* Fix a regression in hooks using the wrong absolute gravity binary path.


### 4.57.0 LTS

#### Bugfixes

* Fix an issue with a cluster possibly sending incorrect agent credentials when connecting to an Ops Center.

### 4.56.0 LTS

#### Improvements

* Make `dnsmasq` listen only on localhost.

### 4.55.0 LTS

#### Improvements

* OS distribution checker retrieves release metadata more consistently.
* Add installation health check to validate d_type support in the backing filesystem for Docker overlay.

#### Bugfixes

* Avoid sporadic block device deactivation on container shutdown for LVM.


### 4.54.0 LTS

#### Improvements

* Ability to override the service user when installing. Read more [here](pack.md#service-user).
* Additional preflight checks during installation and update.
* Remove the 32 characters restriction on the syslog tag in the logging application.


### 4.53.0 LTS

#### Improvements

* Update Kubernetes to version 1.7.12.

### 4.52.0 LTS

#### Bugfixes

* Fix an integration issue with AWS API that was causing keys validation to fail.

### 4.51.0 LTS

#### Improvements

* Stopping the gravity runtime systemd unit is now more stable due to increased service stop timeout.

#### Bugfixes

* Fix agents reconnecting forever if installer shuts down before an agent is able
to query the operation state
* Fix intermittent unexpected closed connection errors during installation of the application
* Fix potential endless loop when observing hook job progress after API server experiences a
transient error

### 4.50.0 LTS

#### Bugfixes

* Fix regression in removing legacy update directory.

### 4.49.0 LTS

#### Improvements

* Add support for configuring docker bridge in promiscuous mode.

#### Bugfixes

* Fix LVM system directory detection on update.
* Remove legacy update directory before the update.

### 4.48.0 LTS

#### Improvements

* Return HTTP status code 503 (service unavailable) from cluster agent HTTPS endpoint for degraded cluster status.

#### Bugfixes

* Fix filesystem matching in volume requirements checker.

### 4.47.0 LTS

#### Improvements

* Add `--timeout` flag to `gravity restore`.
* Add `--follow` flag to backup and restore commands to follow operation logs in real-time.
* Bundle an application manifest with the installer tarball.

#### Bugfixes

* Fix an issue with reverse tunnel reappearing after gravity-site restart.

### 4.46.0 LTS

#### Improvements

* Collect kernel module information as a part of cluster debug report.

### 4.45.0 LTS

#### Bugfixes

* Improved stability of installs on clusters with > 3 nodes.
* Fix monitoring application to load default dashboard/rollups.
* Fix panic in upgrade if initiated on a non-master node.

### 4.44.0 LTS

#### Improvements

* `gravity status` is now more reliable on an unhealthy cluster.
* Add a `--timeout` option to `gravity backup`.

### 4.43.0

#### Improvements

* Configurable monitoring alerts.

#### Bugfixes

* Fix the pre-update upgrade phase to properly depend on init phase.

### 4.42.0

#### Bugfixes

* Fix regression with gravity/tele not building on macOS.

### 4.41.0

!!! warning
	  Due to a regression this build does not include macOS binaries. Please download 4.42.0 if you
	  need macOS binaries.

#### Improvements

* Improve error message in UI for nodes with failed checks to include node name.
* Add support to `gravity resource get` to fetch individual resources.
* Print used temporary directory during install.
* Always pre-select interface in case if only one interface is available.
* Add kernel parameter checks to install prechecks.
* Add support for `tele get apps` command syntax.

#### Bugfixes

* Fix panic in gravity process on corrupted TLS key pairs.
* `tele push ` no longer requires superuser privileges on the client.
* `tele login` uses correct opscenter port for `tsh profile` setup.
* `gravity leave --force` will always work in case if lost connection to the cluster.
* Add escaping for special characters in generated systemd unit names.

### 4.40.0

#### Improvements

* Update planet and kubernetes to include fixes for security vulnerabilities in dnsmasq. Read more [here](https://security.googleblog.com/2017/10/behind-masq-yet-more-dns-and-dhcp.html)

### 4.39.0

#### Bugfixes

* Fix an issue with clean up of systemd units during installation.

### 4.38.0

#### Improvements

* Add support for TLS keypair configuration via resources. Read more [here](config.md#tls-key-pair).
* Simplify Ops Center [post install configuration](hub.md#post-provisioning).

#### Bugfixes

* Properly handle system service failing to stop in system upgrade phase. Include a message to restart the node if the planet service stop failed.
* Fix the upgrade taint phase.
* Fix the kubernetes-specific upgrade phases to properly reference nodes on AWS .

### 4.37.0

#### Improvements

* Add ability to provide a custom directory for system data during install/join. See command references in
  [Automatic Installer](installation.md#cli-installation) and [Adding a Node](cluster.md#adding-a-node) chapters
  for more details.
* Add option to Kubernetes tab in UI to SSH directly into a running container.

### 4.36.0

#### Improvements

* Refine update process with new Kubernetes phases, see [Separation of workloads](cluster.md#separation-of-workloads) for more details.

### 4.35.0

#### Improvements

* Add ability to provide additional command line arguments to etcd and kubelet via application manifest, see [Image Manifest](pack.md#image-manifest) for more details.

### 4.34.0

#### Improvements

* Upgrade to Teleport Enterprise 2.3.
* Add support for advanced RBAC for cluster access via Gravity Hub, see [Cluster RBAC section](hub.md#accessing-gravity-hub)
  for more information.

### 4.32.0

#### Bugfixes

* Fix an issue with DNS service initialization during update.

### 4.31.0

#### Improvements

* Upgrade to Kubernetes 1.7.5.
* Add support for a `logforwarder` resource, see [Configuring Log Forwarders](config.md#log-forwarders)
  for more information.

#### Bugfixes

* Fix an issue with certain fields not being validated when creating a user.

### 4.30.0

#### Improvements

* Add support for `uid`, `gid` and `mode` properties in application manifest `Volume`
  [section](pack.md#image-manifest-format)

### 4.29.0

#### Improvements

* Add support for `gravity upgrade --phase= --force` to retry a failed upgrade fphase.

### 4.27.0

#### Bugfixes

* Fix migrations from 4.17.0 to 4.24.0 that caused remote clusters to go offline after upgrade.

### 4.24.0

#### Improvements

* Make upgrade upload more tolerant to transient issues.
* Internal improvements to improve resiliency of the new upgrade procedure.

### 4.23.0

#### Improvements

* Upgrade to Kubernetes 1.7.4.
* Automatic upgrade procedure has been redesigned to take advantage of the new plan-based upgrade.

### 4.22.0

#### Improvements

* Introduce a redesigned manual upgrade procedure, see [Manual Upgrade Mode](cluster.md#manual-upgrade).

### 4.21.0

#### Improvements

* New `tele create` command creates clusters via the OpsCenter.

### 4.20.0

#### Bugfixes

* Fixes for authentication in hooks

### 4.19.0

#### Improvements

* Add custom provisioning hooks feature.
* Display logs and progress when deprovisioning the cluster.
* Add credentials dialog when deprovisioning the nodes.

### 4.18.0

#### Improvements

* Several security improvements to the platform.

### 4.17.0

#### Bugfixes

* When a user was assigned to multiple Kubernetes groups, only one of them was accounted for in the client certificate obtained by tele login.

### 4.16.0

#### Bugfixes

* Update operation reliability fixes.
* Fix an issue with user resource not being updated during upsert.

#### Improvements

* Make the platform more tolerant to transient etcd errors.

### 4.15.0

#### Bugfixes

* Prevent gravity cluster controller from running busy loops when etcd is misconfigured or unavailable.
* Fix an issue with user-defined volume failing performance precheck if its directory doesn't exist.

### 4.14.0

#### Improvements

* Add support for new resources `user` and `token`. See [Configuring a Cluster](config.md) for details.

### 4.13.0

#### Bugfixes

* Fix `gravity resource` command that was broken after system upgrades.
* Fix output for AWS instances without public IP address.

### 4.12.0

#### Bugfixes

* Fix an issue with incorrect ownership of some files during update operation.

#### Improvements

* Add support for a new resource type `role`. See [Configuring a Cluster](config.md) for details.

### 4.11.0

#### Bugfixes

* Fix integration with Keycloak OIDC provider.

### 4.10.0

#### Bugfixes

* Updating an application could result in failure to find a roles/policies bootstrap configuration.

### 4.9.0

#### Bugfixes

* Fix an issue with clusters not being able to reconnect after Ops Center restart sometimes.
* Fix an issue with the logs tab not working properly in deployed clusters.

#### Improvements

* Multiple stability fixes for install/expand operations.

### 4.8.0

#### Improvements

* Introduce a set of `gravity resource` commands for cluster resources management (currently, [only OIDC connectors](config.md)).

### 4.7.0

#### Security

* Enable Kubernetes [Pod Security Policies](https://kubernetes.io/docs/concepts/policy/pod-security-policy/) for Gravity clusters.

### 4.6.0

#### Improvements

* More consistent collection of diagnostics
* Diagnostics collection command renamed to `gravity report`

### 4.5.0

#### Security

* Tighten up security across the whole platform for compliance with [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes/).

### 4.4.0

#### Bugfixes

* Clusters can appear offline in case if Ops Center is deployed in HA mode.

#### Improvements

* Update teleport to 2.2.3

### 4.3.0

#### Improvements

* Add support for using AWS Session tokens when provisioning.

### 4.2.0

#### Bugfixes

* When accessing cluster page via Ops Center, show only downloaded version upgrades.
* Fix an issue with all nodes becoming full etcd members during initial install.
* Install log UI fixes around overflow and scrolling.

#### Improvements

* Better validation for Docker storage driver specified in manifest or provided via CLI flag, and semver.
* Extend pre-installation checks to check /tmp and more conflicting processes.
* Better connection loss handing during install operation.
* Wait for server to expire from backend before completing shrink operation.

### 4.1.0

#### Bugfixes

* Joined nodes were configured with incorrect pod/service subnets if custom subnets were used during initial install.

#### Features

* Add ability to override Docker storage driver via `--storage-driver` CLI parameter to `gravity install` command.
* Allow up to 5 nodes to be joining the cluster simultaneously for 5+ node clusters.

#### Reliability

* When installing application, use polling instead of a long-running blocking network call.

### 4.0.0

#### Bugfixes

* Downloaded apps/packages are upserted during installation to make retries on connection failures idempotent.

## 3.x Releases

### 3.64.0 LTS

#### Bugfixes

* Revert partial security changes

### 3.63.0 LTS

#### Bugfixes

* Fix an integration issue with AWS API that was causing keys validation to fail.

### 3.62.0

#### Bugfixes

* Fix an issue with Telegraf not being able to authenticate with InfluxDB.
* Fix an issue with Heapster using Kubernetes config with insufficient permissions.

#### Improvements

* Upgrade Heapster to v1.4.1.

### 3.61.0

#### Improvements

* Successful backup jobs are now automatically cleaned up.

#### Bugfixes

* Fix an issue with handling configuration packages during system upgrade.

### 3.60.0

#### Bugfixes

* Fix an issue with configuration packages lookup when upgrading from older versions.

### 3.59.0

#### Security

* Gravity security audit fixes

### 3.58.0

#### Security

* Tighten up security across the whole platform for compliance with [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes/).

### 3.54.0

#### Bugfixes

* Fixed typo in installer screen.
* Ignore heartbeats with error.
* Fixed etcd + lvm issues for install/expand/upgrade.
* Restrict unauthorized access to kubelet.

### 3.53.0

#### Bugfixes

* Install log UI fixes around overflow and scrolling.

### 3.52.0

#### Bugfixes

* Grafana dashboards now readonly.

### 3.51.0

Gravity 3.51.0 is **LTS release supported for 1 year** for our enterprise customers with EOL of June, 2nd 2018.

It contains a number of improvements and bugfixes.

#### Bugfixes

* Cluster update improvements - stability bugfixes

#### Features

* Standalone wizard link auto expires in 4 hours
* Update github.com/gravitational/monitoring-app to 3.1.0
* Upgrade Kubernetes to 1.5.7

### 3.50.0

#### Bugfixes

* Expand operation stays in progress forever it it fails to start

#### Features

* Use --force when tagging images to fix `tele build` with older docker versions.
* Periodic registry sync on HA Docker registry improves [registry stability](https://gravitational.zendesk.com/hc/en-us/articles/115008202348)
* Update InfluxDB to [1.2.2](https://github.com/influxdata/influxdb/blob/master/CHANGELOG.md#v122-2017-03-14)

### 3.48.0

#### Features

* Fix grammar in documentation

* Update github.com/gravitational/monitoring-app to 2.8.0 that includes alerting
* Fixing update processing indicator

### 3.45.0

### Bugfixes

* Use dns to access gravity-site when running non-kube-system hooks

### 3.43.0

#### Bugfixes

* Enable network.target [dependency for planet](https://gravitational.zendesk.com/hc/en-us/articles/115008045447)

### 3.42.0

#### Features

* Add 5 and 6 node flavors to gravity bundle
* Don't exit interactive installer after failure. This will allow to collect crashreports via UI.

### 3.41.0

### Features

* Add lvm system directory to agent heartbeat to improve install experience.

### 3.40.0

### Bugfixes

* Use the container image implicitly referring to private docker registry to fix air-gapped installs.
* Fix docker volume speed detection issue. Too small blocks caused incorrect disk speed assessment on Azure.

### 3.38.0

#### Bugfixes

* Specify hook job namespace when we create it to fix user application hooks.

#### Features

* Add example of getting the gravity binaries of a certain version
* Update to teleport 2.0.6

### 3.37.0

#### Features

* Add ability to specify pod/service network CIDR range via UI and CLI
* Add AWS IAM policy to the [docs](requirements.md#aws-iam-policy)
* Add runbook to recover the cluster after node failure [docs](cluster.md#recovering-a-node)

### 3.36.0

#### Bugfixes

* When logging in with one-time token, redirect to appropriate installer page
* Use simpler command for lvm check
* Unify remote commands logging and add server IP
* Fix out-of-order progress step reporting during operations (@sofuture)
* Additional check for release version for RHEL-based OS

### 3.35.0

#### Bugfixes

* Expose lvm state from host inside planet
* Fix vendoring images without explicit tag

### 3.34.0

#### Bugfixes

* Retry app sync on network errors

### 3.32.0

#### Features

* Explicitly create a log sub-directory for journald to preserve logs between restarts
* Change tar.gz to tar in the docs

### 3.30.0

#### Bugfixes

* Fix issue with multiple shrink operations causing deadlock

### 3.29.0

#### Features

* 10x speed up for tele pull command

### 3.28.0

#### Features

* Better output for app import

### 3.27.0

#### Bugfixes

* Auto-deprovision nodes after failed one-time link install
* Copy file permissions in addition to contents in bundles

### 3.26.0

#### Features

* Add ca-cert and encryption key to tele build

### 3.24.0

#### Features

* Add TLS settings page
* Check for docker before tele build

### 3.23.0

#### Features

* Add hooks docs

#### Bugfixes

* Use state-dir in same place for all tele commands
* Do not ignore system errors when opening application manifest file

### 3.22.0

#### Features

* Document the status hook
* Reduce number of required cores for opscenter to 2
* Disable local state precheck

#### Bugfixes

* Fix for self-destruct button
* More visible errors during install/expand

### 3.21.0

#### Features

* Add self-destruct button to cluster connected to the ops center

### 3.17.0

#### Bugfixes

* Fix enabling remote access toggle

### 3.16.0

#### Bugfixes

* Fix logs for daemons and check clocks on servers to be in sync
* Do not silently ignore errors when creating update resources

### 3.14.0

#### Features

#### Bugfixes

* Reset op state when prechecks fail

### 3.13.0

#### Features

* Use overlay for gravity-app by default

#### Bugfixes

* Successful update reverts gravity-site to previous version
* client: use server ID to refer to nodes

### 3.12.0

### Bugfixes

* Remove / and /tmp prechecks blocking some installs

### 3.11.0

#### Features

* Add docs section on deleting a cluster

### 3.8.0

#### Bugfixes

* Return VPC tags in agent heartbeat
* Availability zone retries on cluster provisioning

### 3.7.0

#### Features

* Add X-Forwarded-* headers to proxy forwards

#### Bugfixes

* Remove the wizard reverse tunnel after the installation so the cluster

### 3.6.0

#### Features

* Backup/restore improvements


### 3.5.0

#### Features

* Update kubernetes to v1.5.3
* Better AWS permissions error reporting

#### Bugfixes

* Expand and update operation certificate fixes
* Update teleport with fixed session timeouts

### 3.0.0

#### Features

* Update manifest docs

#### Bugfixes

* Should not display progress status if operation failed
* Ignore already exists error when pulling apps
* Check for hostname uniqueness before joining agent

## 2.x Releases

### 2.10.0

#### Bugfixes

* Control leader notifications arriving faster than app export
* Use pod IP as process ID
* give gravity-admin admin on default namespace
* Remove name check for gravity binaries


## 1.x Releases

### 1.29.0

#### Features

* Improve speed of gravity app unpack command to speedup app hooks

#### Bugfixes

* Stop status check and other services when gravity loses leadership


### 1.26.0

### Features

* Add opscenter section to documentation

### 1.25.0

### Bugfixes

* Fix trusted authorities ACL method


### 1.22.0

### Bugfixes

* Delete tunnel after the install
* Fix a channel test that would intermittently block


### 1.24.0

#### Features

* Check tele/runtime version compatibility on build

### 1.23.0

#### Bugfixes

* Remove ls/sh commands and their docs
