---
title: Gravity Releases (Changelog)
description: List of Gravity releases and changes between them.
---

# Releases

Find the latest Open Source Gravity releases at [Gravity Downloads](https://gravitational.com/gravity/download).

## Supported Versions

| Version             | Latest Patch  | LTS | Release Date         | Latest Patch Date    | End of Support *        | Kubernetes Version   | Teleport Version |
| ------------------- | ------------- | --- | -------------------- | -------------------- | ----------------------- | -------------------- | ---------------- |
| [9.0](#90-releases) | 9.0.0-alpha.0 | No  | pre-release          | April 21, 2021       | Set upon release        | 1.21.0               | 3.2.17-gravity   |
| [8.0](#80-releases) | 8.0.0-alpha.0 | No  | pre-release          | April 19, 2021       | Set upon release        | 1.19.8               | 3.2.17-gravity   |
| [7.1](#71-releases) | 7.1.0-alpha.6 | No  | pre-release          | April 14, 2021       | Set upon release        | 1.19.8               | 3.2.17-gravity   |
| [7.0](#70-releases) | 7.0.31        | Yes | April 3, 2020        | April 15, 2021       | July 9, 2022            | 1.17.9               | 3.2.14-gravity   |
| [6.1](#61-releases) | 6.1.48        | Yes | August 2, 2019       | March 23, 2021       | November 10, 2021       | 1.15.12              | 3.2.14-gravity   |
| [5.5](#55-releases) | 5.5.59        | Yes | March 8, 2019        | March 30, 2021       | March 8, 2021           | 1.13.11              | 3.0.7-gravity    |

Gravity offers one Long Term Support (LTS) version for every 2nd Kubernetes
minor version, allowing for seamless upgrades per Kubernetes
[supported version skew policy](https://kubernetes.io/docs/setup/release/version-skew-policy/#supported-version-skew).
Gravity LTS versions are supported with security and bug fixes for two years.

Non-LTS (regular) branches of Gravity offer the latest features and include
more current versions of Kubernetes. Regular branches of Gravity are supported
with security and bug fixes until first release of the subsequent Gravity
branch is published.

_* Unless extended through customer agreement._

## Unsupported Versions

These versions are past their End of Support date, and no longer receive security
and bug fixes. [Gravity customers](https://gravitational.com/gravity/demo/) can
extend updates past End of Support through customer agreements if required.

| Version             | Latest Patch | LTS | Release Date         | End of Support          | Kubernetes Version   | Teleport Version |
| ------------------- | ------------ | --- | -------------------- | ----------------------- | -------------------- | ---------------- |
| [6.3](#63-releases) | 6.3.18       | No  | December 18, 2019    | April 3, 2020 (7.0)     | 1.17.6               | 3.2.13           |
| [6.2](#62-releases) | 6.2.5        | No  | September 24, 2019   | December 18, 2019 (6.3) | 1.16.3               | 3.2.13           |
| [6.0](#60-releases) | 6.0.10       | No  | July 17, 2019        | August 2, 2019 (6.1)    | 1.14.7               | 3.2.12           |
| [5.6](#56-releases) | 5.6.8        | No  | April 19, 2019       | July 17, 2019 (6.0)     | 1.14.7               | 3.0.6-gravity    |
| [5.4](#54-releases) | 5.4.10       | No  | December 14, 2018    | March 8, 2019 (5.5)     | 1.13.5               | 2.4.10           |
| [5.3](#53-releases) | 5.3.9        | No  | October 19, 2018     | December 14, 2018 (5.4) | 1.12.3               | 2.4.7            |
| [5.2](#52-releases) | 5.2.18       | Yes | October 15, 2018     | October 15, 2019        | 1.11.9               | 2.4.10           |
| [5.0](#50-releases) | 5.0.36       | Yes | April 18, 2018       | April 13, 2019          | 1.9.13-gravitational | 2.4.10           |
| [4.x](#4x-releases) | 4.68.0       | Yes | June 1, 2017         | November 16, 2018       | 1.7.18-gravitational | 2.3.5            |
| [3.x](#3x-releases) | 3.64.0       | Yes | February 16, 2017    | June 2, 2018            | 1.5.7                | 2.0.6            |
| [1.x](#1x-releases) | 1.29.0       | Yes | November 2nd, 2016   | March 21, 2018          | 1.3.8                | 1.2.0            |

# Release Notes

## 9.0 Releases

9.0 is currently pre-release.

### 9.0.0-alpha.0 (April 21, 2021)

#### Improvements
* Add support for Kubernetes 1.21 ([#2471](https://github.com/gravitational/gravity/pull/2471), [planet#836](https://github.com/gravitational/planet/pull/836)).

## 8.0 Releases

8.0 is currently pre-release.

### 8.0.0-alpha.0 (April 19, 2021)

This version is equivalent to 7.1.0-alpha.6.

## 7.1 Releases

7.1 is currently pre-release.

### 7.1.0-alpha.6 (April 14, 2021)

#### Improvements
* Add GCE Alias IP support for flannel ([#2439](https://github.com/gravitational/gravity/pull/2439), [planet#830](https://github.com/gravitational/planet/pull/830), [flannel#10](https://github.com/gravitational/flannel/pull/10)).
* Improve performance of AWS autoscaling integration ([#2458](https://github.com/gravitational/gravity/pull/2458)).
* Improve satellite cluster health monitoring on large clusters up to 1000 nodes ([#2439](https://github.com/gravitational/gravity/pull/2439), [planet#828](https://github.com/gravitational/planet/pull/828), [satellite#297](https://github.com/gravitational/satellite/pull/297)).

#### Bugfixes
* Fix ownership of monitoring and logrange directories ([#2448](https://github.com/gravitational/gravity/pull/2448)).
* Remove `gravity app sync`'s dependence on a kubernetes config ([#2438](https://github.com/gravitational/gravity/pull/2438)).

#### Internal Changes
* Upgrade internal kubernetes client go to v1.19.8 and helm to v3.4.2 ([#2433](https://github.com/gravitational/gravity/pull/2433), [planet#827](https://github.com/gravitational/planet/pull/827), [rigging#100](https://github.com/gravitational/rigging/pull/100), [satellite#299](https://github.com/gravitational/satellite/pull/299)).
* Improve build and release processes ([#2451](https://github.com/gravitational/gravity/pull/2451), [#2466](https://github.com/gravitational/gravity/pull/2466)).

### 7.1.0-alpha.5 (March 16, 2021)

All changes listed are in comparison to 7.0.30 LTS.

#### Improvements
* Open source all Gravity code previously limited to enterprise customers! ([#2375](https://github.com/gravitational/gravity/pull/2375))
* Add support for Kubernetes 1.19 ([#2425](https://github.com/gravitational/gravity/pull/2425), [planet#825](https://github.com/gravitational/planet/pull/825), [storage-app#5](https://github.com/gravitational/storage-app/pull/5), [logging-app#76](https://github.com/gravitational/logging-app/pull/76), [monitoring-app#206](https://github.com/gravitational/monitoring-app/pull/206), [bandwagon#33](https://github.com/gravitational/bandwagon/pull/33))
* Add an Ingress App to gravity ([#1435](https://github.com/gravitational/gravity/pull/1435))
* Allow OpsCenter and the Web Installation wizard to be disabled in the cluster manifest ([#2371](https://github.com/gravitational/gravity/pull/2371))
* Allow Logging, Monitoring, Ingress, Tiller, Storage, and Bandwagon applications to be disabled in the cluster manifest ([#2397](https://github.com/gravitational/gravity/pull/2397))
* Expose additional logrange fields in logging-app ([#2369](https://github.com/gravitational/gravity/pull/2369), [logging-app#75](https://github.com/gravitational/logging-app/pull/75))
* Many scaleability tweaks for large clusters ([#2426](https://github.com/gravitational/gravity/pull/2426)) including:
    * Introduce a hidden flag that eliminates workers from upgrade plan
    * Introduce a hidden flag to allow parallel execution of certain upgrade phases
    * Cache plan phases instead of re-querying Etcd each time
    * Parallelize phases that run on multiple nodes, but do not affect the cluster (e.g. configuriation)
    * Improve AWS event handler integration
    * Add debug logging (including response times) to our internal API client
    * Pre-emptively exit if an expand operation won't be accepted
    * Allow multiple worker shrink/expand operations to run at the same time
* Split `tele helm build` from `tele build` ([#1317](https://github.com/gravitational/gravity/pull/1317))
* Bump Helm 2 version to 2.16.12 ([#2307](https://github.com/gravitational/gravity/pull/2307), [#2310](https://github.com/gravitational/gravity/pull/2310), [planet#789](https://github.com/gravitational/planet/pull/789))

#### Removals
* Remove support for Debian 8 ([#2210](https://github.com/gravitational/gravity/pull/2210))
* Remove provisioner resources from the reference `telekube` application image ([#1561](https://github.com/gravitational/gravity/pull/1561))
* Remove the Gravity's terraform provider ([#1622](https://github.com/gravitational/gravity/pull/1622))

#### Bugfixes
* Fix an issue with hub certificates missing SANs after expand ([#1323](https://github.com/gravitational/gravity/pull/1323)).
* Remove stack traces from web API calls ([#2357](https://github.com/gravitational/gravity/pull/2357), [teleport#5070](https://github.com/gravitational/teleport/pull/5070), [trace#60](https://github.com/gravitational/trace/pull/60))
* Fix a bug where tailing an operation plan would truncate at 100 events ([#2426](https://github.com/gravitational/gravity/pull/2426))

#### Internal Changes
* Increase CoreDNS CPU and memory allocation [#2430](https://github.com/gravitational/gravity/pull/2430)
* Replace in-house drain logic with kubernetes drain logic ([#1652](https://github.com/gravitational/gravity/pull/1652))
* Experimentally allow Kubernetes High Availability mode to be enabled in the cluster manifest ([#2404](https://github.com/gravitational/gravity/pull/2404), [planet#821](https://github.com/gravitational/planet/pull/821))
* Remove [Serf](https://www.serf.io/) from the ping/latency checker ([#2234](https://github.com/gravitational/gravity/pull/2234), [planet#765](https://github.com/gravitational/planet/pull/765), [satellite#277](https://github.com/gravitational/satellite/pull/277), [monitoring-app#197](https://github.com/gravitational/monitoring-app/pull/197))
* Replace [Serf](https://www.serf.io/) for managing cluster membership ([#2329](https://github.com/gravitational/gravity/pull/2329), [planet#794](https://github.com/gravitational/planet/pull/794), [satellite#284](https://github.com/gravitational/satellite/pull/284), [satellite#282](https://github.com/gravitational/satellite/pull/282))
* Switch from [Dep](https://github.com/golang/dep) to [Go Modules](https://golang.org/ref/mod) ([#1454](https://github.com/gravitational/gravity/pull/1454))
* And many other minor code cleanup and build improvements.


## 7.0 Releases

### 7.0.31 LTS (April 15, 2021)

#### Improvements
* Add GCE Alias IP support for flannel ([#2440](https://github.com/gravitational/gravity/pull/2440), [planet#831](https://github.com/gravitational/planet/pull/831), [flannel#10](https://github.com/gravitational/flannel/pull/10)).
* Expose additional logrange fields in logging-app ([#2403](https://github.com/gravitational/gravity/pull/2403), [logging-app#75](https://github.com/gravitational/logging-app/pull/75)).

#### Bugfixes
* Remove stack traces from web API calls ([#2468](https://github.com/gravitational/gravity/pull/2468), [teleport#5070](https://github.com/gravitational/teleport/pull/5070), [trace#60](https://github.com/gravitational/trace/pull/60)).
* Set a high planet pid limit, instead of the sometimes ignored 'unlimited pids' ([#2470](https://github.com/gravitational/gravity/pull/2470), [#834](https://github.com/gravitational/planet/pull/834), [#2444](https://github.com/gravitational/gravity/issues/2444)).
* Fix a logging-app panic ([#2464](https://github.com/gravitational/gravity/pull/2464), [logging-app#79](https://github.com/gravitational/logging-app/pull/79)).
* Fix an error while gathering mount points in monitoring-app ([#2452](https://github.com/gravitational/gravity/pull/2452), [monitoring-app#208](https://github.com/gravitational/monitoring-app/pull/208)).
* Fix an error where `gravity plan` would occasionally display the wrong operation ([#2412](https://github.com/gravitational/gravity/pull/2412), [#2309](https://github.com/gravitational/gravity/issues/2309)).

#### Internal Changes
* Change ownership of logging-app data ([#2447](https://github.com/gravitational/gravity/pull/2447), [#2464](https://github.com/gravitational/gravity/pull/2464), [logging-app#79](https://github.com/gravitational/logging-app/pull/79)).
* Experimentally allow Kubernetes High Availability mode to be enabled in the cluster manifest ([#2411](https://github.com/gravitational/gravity/pull/2411), [planet#823](https://github.com/gravitational/planet/pull/823)).

!!! warning
    Upgrading to this release will cause logging-app abandon root owned data in `/var/lib/gravity/logrange/lr` and `/var/lib/gravity/logrange/data`.
    This data needs to be cleaned by hand.

### 7.0.30 LTS (January 15, 2021)

#### Bugfixes
* Fix an issue with podAntiAffinity on kube-dns workers ([#2387](https://github.com/gravitational/gravity/pull/2387)).

### 7.0.29 LTS (January 9, 2021)

#### Improvements
* Add limited support for specifying additional kubernetes admission controllers ([#2367](https://github.com/gravitational/gravity/pull/2367), [planet#810](https://github.com/gravitational/planet/pull/810)).

#### Bugfixes
* Fix an issue with file descriptors leaking in monitoring network health ([#2383](https://github.com/gravitational/gravity/pull/2383), [monitoring-app#204](https://github.com/gravitational/monitoring-app/pull/204), [satellite#294](https://github.com/gravitational/satellite/pull/294)).

### 7.0.28 LTS (December 7, 2020)

#### Improvements
* Add helm 3 binary to planet and make it available for hooks ([#2345](https://github.com/gravitational/gravity/pull/2345)).
* Add support for using a separate network project in GCE to flannel ([#2355](https://github.com/gravitational/gravity/pull/2355), [planet#807](https://github.com/gravitational/planet/pull/807), [flannel#9](https://github.com/gravitational/satellite/pull/9)).

#### Bugfixes
* Shutdown kubernetes control plane immediately when elections are disabled ([#2355](https://github.com/gravitational/gravity/pull/2355), [planet#805](https://github.com/gravitational/planet/pull/805)).

### 7.0.27 LTS (November 23, 2020)

#### Improvements
* Add support for Redhat/Centos 7.9 and 8.3 ([#2335](https://github.com/gravitational/gravity/pull/2335), [planet#796](https://github.com/gravitational/planet/pull/796), [satellite#286](https://github.com/gravitational/satellite/pull/286)).
* Update Helm/Tiller to 2.16.12 ([#2319](https://github.com/gravitational/gravity/pull/2319), [planet#793](https://github.com/gravitational/planet/pull/793)).

#### Bugfixes

* Fix an issue where gravity wouldn't be able to generate an upgrade plan due to corrupted planet metadata ([#2344](https://github.com/gravitational/gravity/pull/2344)).
* Fix an issue where monitoring-app would not tolerate customer applied taints ([#2340](https://github.com/gravitational/gravity/pull/2340), [monitoring-app#199](https://github.com/gravitational/monitoring-app/pull/199)).
* Fix an issue where logging-app would not tolerate customer applied taints ([#2340](https://github.com/gravitational/gravity/pull/2340), [logging-app#74](https://github.com/gravitational/logging-app/pull/74)).
* Increase the amount of time dns-app update hooks will wait for cluster changes to complete ([#2326](https://github.com/gravitational/gravity/pull/2326)).
* Increase FD limits when creating systemd units ([#2321](https://github.com/gravitational/gravity/pull/2321)).

### 7.0.26 LTS (November 11, 2020)

#### Bugfixes

* Fix an issue with planet agent failing to provide correct list of nameservers to kubelet on worker nodes ([#2312](https://github.com/gravitational/gravity/pull/2312), [planet#791](https://github.com/gravitational/planet/pull/791)).

### 7.0.25 LTS (November 4, 2020)

#### Improvements

* Allow configuration of the Pod subnet size through the Cluster Configuration resource ([#2302](https://github.com/gravitational/gravity/pull/2302), [planet#785](https://github.com/gravitational/planet/pull/785)).

#### Bugfixes

* Fix an issue where `gravity stop` fails when executed with a custom planet package ([#2191](https://github.com/gravitational/gravity/pull/2191)).


### 7.0.24 LTS (November 2, 2020)

#### Improvements

* [Allow customization of number of CoreDNS worker instances.](https://gravitational.com/gravity/docs/config/#customize-number-of-dns-instances-on-workers) ([#2299](https://github.com/gravitational/gravity/pull/2299)) ([rigging#83](https://github.com/gravitational/rigging/pull/83)).

#### Bugfixes

* Fix an issue when install is triggered outside of the extracted directory ([#2295](https://github.com/gravitational/gravity/pull/2295)).
* Fix an issue where tele build could select the wrong runtime version ([#2247](https://github.com/gravitational/gravity/pull/2247)).
* Fix an issue where self signed certificates are not accepted by latest MacOS requirements ([#2279](https://github.com/gravitational/gravity/pull/2279)).


### 7.0.23 LTS (October 26, 2020)

#### Bugfixes

* Fix an issue with multi-hop upgrades that involve multiple etcd upgrades ([#2275](https://github.com/gravitational/gravity/pull/2275)). 

### 7.0.22 LTS (October 23rd, 2020)

#### Bugfixes

* Make it possible to install/expand/upgrade clusters based on CentOS/rhel 8 with SELinux in enforcing mode ([#2240](https://github.com/gravitational/gravity/pull/2240)).

### 7.0.21 LTS (October 22nd, 2020)

#### Improvements

* Update TLS cipher suites for Kubernetes components ([#2265](https://github.com/gravitational/gravity/pull/2265), [planet#782](https://github.com/gravitational/planet/pull/782)).
* Satellite queries for system pods will use less load by searching only the `kube-system` and `monitoring` namespaces for critical pods that aren't running ([#2250](https://github.com/gravitational/gravity/pull/2250), [planet#775](https://github.com/gravitational/planet/pull/775), [satellite#281](https://github.com/gravitational/satellite/pull/281)).
* Implement a check to prevent installation if the chosen overlay network conflicts with the host networking ([#2204](https://github.com/gravitational/gravity/pull/2204)).
* Implement a check so that a phase is only rolled back if dependent phases have also been rolled back ([#2219](https://github.com/gravitational/gravity/pull/2219)).
* Increase base value of expand concurrency to allow a minimum of 4 simultaneous worker joins ([#2205](https://github.com/gravitational/gravity/pull/2205)).
* Include additional cloud metadata as part of `gravity status` output ([#2202](https://github.com/gravitational/gravity/pull/2202)).

#### Bugfixes

* Fix an issue where flannel could corrupt iptables rules if newly generated rules don't exactly match rules previously used ([#2265](https://github.com/gravitational/gravity/pull/2265), [planet#777](https://github.com/gravitational/planet/pull/777), [flannel#7](https://github.com/gravitational/flannel/pull/7)).
* Fix an issue when using GCE integrations that unnecessary OAuth scopes would be requested ([#2265](https://github.com/gravitational/gravity/pull/2265), [planet#777](https://github.com/gravitational/planet/pull/777), [flannel#7](https://github.com/gravitational/flannel/pull/8)).
* Fix an issue where etcd-backups were using too short of a timer ([#2250](https://github.com/gravitational/gravity/pull/2250), [planet#775](https://github.com/gravitational/planet/pull/775), [etcd-backup#5](https://github.com/gravitational/satellite/pull/5)).
* Fix an issue where cluster configuration could be lost during validation ([#2253](https://github.com/gravitational/gravity/pull/2253)).
* Fix a panic in the audit event emitter ([#2241](https://github.com/gravitational/gravity/pull/2241)).
* Fix an issue that prevents log truncation ([#2238](https://github.com/gravitational/gravity/pull/2238), [logging-app#72](https://github.com/gravitational/logging-app/pull/72)).


### 7.0.20 LTS (October 9th, 2020)

#### Bugfixes

* Fix a regression in `tele build` that failed to handle direct package dependencies in a cluster manifest ([#2211](https://github.com/gravitational/gravity/pull/2211)).

### 7.0.19 LTS (October 6th, 2020)

#### Improvements
* Update a minimum required subnet size for pod network cidr to be a /22 ([#2182](https://github.com/gravitational/gravity/pull/2182)).

#### Bugfixes

* Fix a security issue when using SAML authentication to an identity provider (CVE-2020-15216) ([#2193](https://github.com/gravitational/gravity/pull/2193)).
* Fix an issue with serf clusters missing nodes when partitioned for more than 24 hours ([#2027](https://github.com/gravitational/gravity/pull/2027), [planet#760](https://github.com/gravitational/planet/pull/760)).

!!! warning
    This release fixes a security vulnerability in Teleport when connecting Gravity to a SAML 2.0 identity provider. Please see
    [Teleport Announcement for CVE-2020-15216](https://github.com/gravitational/teleport/releases/tag/v4.3.7) for more information.

### 7.0.18 LTS (October 1st, 2020)

#### Improvements

* Remove redundant default planet container package when using a custom container ([#1982](https://github.com/gravitational/gravity/pull/1982)).

#### Bugfixes

* Fix an issue with preflight checks not accounting for mount overrides ([#2171](https://github.com/gravitational/gravity/pull/2171)).
* Fix an issue with the upgrade unable to determine existing etcd server version when using custom containers  ([#1982](https://github.com/gravitational/gravity/pull/1982)).

### 7.0.17 LTS (September 30th, 2020)

#### Improvements

* Tune gravity to support up to 1000 nodes ([#2159](https://github.com/gravitational/gravity/pull/2159)).
* Introduce reconciliation loop to disable source/dest check when using AWS integrations ([#2163](https://github.com/gravitational/gravity/pull/2163)).
* Include additional gravity configuration within debug reports ([#2162](https://github.com/gravitational/gravity/pull/2162)).
* Verify agents are active before resuming operations ([#2073](https://github.com/gravitational/gravity/pull/2073)).
* Shrink operation resiliency improvements ([#2114](https://github.com/gravitational/gravity/issues/2114)).
* Optimize etcd upgrade to avoid steps that are no longer required ([#2098](https://github.com/gravitational/gravity/issues/2098)).
* Add check that state dir has expected uid/gid set ([#2058](https://github.com/gravitational/gravity/pull/2058)).
* Add check that env variables for http(s) proxy are the correct format ([#2062](https://github.com/gravitational/gravity/pull/2062)).

#### Bugfixes

* Fix an issue with operations stalling when gravity-site or etcd is down ([#2166](https://github.com/gravitational/gravity/pull/2166)).
* Fix an issue with wizard installation ([#2137](https://github.com/gravitational/gravity/pull/2137)).
* Fix an issue with a cluster reporting as online when some nodes have been partitioned ([#2131](https://github.com/gravitational/gravity/pull/2131)).
* Fix an issue with syncing to the registry when using custom planet containers ([#2080](https://github.com/gravitational/gravity/pull/2080)).
* Fix an issue with selinux installs rhel/centos 7 ([#2012](https://github.com/gravitational/gravity/pull/2012)).

### 7.0.16 LTS (August 31st, 2020)

#### Bugfixes

* Fix an issue with disconnected Teleport nodes when rotating all masters within the cluster ([#2029](https://github.com/gravitational/gravity/pull/2029)).
* Fix an issue with installation sometimes failing when trying to install a cluster license ([#2037](https://github.com/gravitational/gravity/pull/2037)).
* Fix an issue with priority class incorrectly being set on the hook jobs ([#2066](https://github.com/gravitational/gravity/pull/2066)).

### 7.0.15 LTS (August 19th, 2020)

#### Improvements

* Increase default `LimitNOFile` parameter within planet's systemd to `655350` ([#2027](https://github.com/gravitational/gravity/pull/2027), [planet#725](https://github.com/gravitational/planet/pull/725)).

#### Bugfixes

* Fix an issue with DNS application priority class not having any effect ([#1989](https://github.com/gravitational/gravity/pull/1989)).

### 7.0.14 LTS (August 7th, 2020)

#### Improvements

* Monitoring application has been updated to version `7.0.3` ([#1893](https://github.com/gravitational/gravity/pull/1893), [monitoring#183](https://github.com/gravitational/monitoring-app/pull/183)).
* Gravity debug report will now include `gravity` CLI command history ([#1861](https://github.com/gravitational/gravity/pull/1861)).
* Gravity will now report a warning if installed kernel is older than the recommended `3.10.0-1127` version ([#1926](https://github.com/gravitational/gravity/pull/1926), [satellite#250](https://github.com/gravitational/satellite/pull/250), [planet#713](https://github.com/gravitational/planet/pull/713)).
* Update `gravity plan resume` command to launch from a systemd service by default, `--block` flag is supported for old behavior ([#1936](https://github.com/gravitational/gravity/pull/1936)).
* Add ability to display deployed agent status using `gravity agent status` command ([#1956](https://github.com/gravitational/gravity/pull/1956)).
* Add support for multi-hop upgrades from 5.5 releases, see [Multi-Hop Upgrades](cluster.md#multi-hop-upgrades) for more details ([#1920](https://github.com/gravitational/gravity/pull/1920), [planet#714](https://github.com/gravitational/planet/pull/714)).

#### Bugfixes

* Rotate RPC credentials on cluster upgrade ([#1749](https://github.com/gravitational/gravity/pull/1749)).
* Allow invocation of the upgrade's `upload` script from external directories ([#1892](https://github.com/gravitational/gravity/pull/1892)).
* Fix an issue with installer failing when a directory for mountpoint does not exist ([#1958](https://github.com/gravitational/gravity/pull/1958), [satellite#252](https://github.com/gravitational/satellite/pull/252)).
* Update `ClusterConfiguration` resource to properly support service CIDR changes ([#1975](https://github.com/gravitational/gravity/pull/1975), [planet#722](https://github.com/gravitational/planet/pull/722)).

### 7.0.13 LTS (July 15th, 2020)

#### Improvements

* Pull only required packages during join ([#1862](https://github.com/gravitational/gravity/pull/1862)).

#### Bugfixes

* Update Kubernetes to 1.17.9 (CVE-2020-8557, CVE-2020-8559) ([#1886](https://github.com/gravitational/gravity/pull/1886), [planet#703](https://github.com/gravitational/planet/pull/703)).

!!! warning
    This release fixes a security vulnerability in Kubernetes. Please see
    [Kubernetes Announcement for CVE-2020-8557](https://groups.google.com/g/kubernetes-announce/c/YCBo2a3wCtU) and [Kubernetes Announcement for CVE-2020-8559](https://groups.google.com/g/kubernetes-announce/c/44da1m3evoU) for more information.



### 7.0.12 LTS (July 10th, 2020)

#### Improvements

* Update `gravity status` to display unhealthy critical system pods ([#1753](https://github.com/gravitational/gravity/pull/1753), [planet#691](https://github.com/gravitational/planet/pull/691), [satellite#236](https://github.com/gravitational/satellite/pull/236)).
* Add `--since` flag to `gravity report` command to allow filtering collected logs by time ([#1806](https://github.com/gravitational/gravity/pull/1806)).
* Add more system information collectors to `gravity report` ([#1806](https://github.com/gravitational/gravity/pull/1806)).
* Set priority classes on critical system pods ([#1814](https://github.com/gravitational/gravity/pull/1814), [logging#66](https://github.com/gravitational/logging-app/pull/66), [monitoring#181](https://github.com/gravitational/monitoring-app/pull/181)).
* Update etcd disk check to produce a warning on soft limits (`50ms` latency, `50` IOPS) and a critical failure on hard limits (`150ms` latency, `10` IOPS) ([#1847](https://github.com/gravitational/gravity/pull/1847)).

#### Bugfixes

* Make sure upgrade is resumed on the correct node to avoid issues with plan inconsistency ([#1794](https://github.com/gravitational/gravity/pull/1794)).
* Make sure upgrade can't be launched if the previous upgrade wasn't completed or fully rolled back ([#1803](https://github.com/gravitational/gravity/pull/1803)).
* Fix an issue with upgrade failing to adjust volume permissions if only uid is specified ([#1803](https://github.com/gravitational/gravity/pull/1803)).
* Fix an issue with some tabs not working when accessed through wizard installer user interface ([#1815](https://github.com/gravitational/gravity/pull/1815)).
* Fix an issue with installation failing on Vultr ([#1815](https://github.com/gravitational/gravity/pull/1815)).
* Fix an issue with installed Helm releases not displayed on the dashboard ([#1815](https://github.com/gravitational/gravity/pull/1815)).
* Fix an issue with signup/reset links not honoring configured public address ([#1815](https://github.com/gravitational/gravity/pull/1815)).
* Fix an issue with `tsh` sometimes producing access denied errors ([#1836](https://github.com/gravitational/gravity/pull/1836)).

### 7.0.11 (June 24th, 2020)

#### Improvements

* Upgrade Grafana to `v6.7.4` ([#1763](https://github.com/gravitational/gravity/pull/1763), [monitoring-app#178](https://github.com/gravitational/monitoring-app/pull/178)).

#### Bugfixes

* Fix a regression with Gravity trying to execute disk space check for volumes that don't specify disk space requirements ([#1758](https://github.com/gravitational/gravity/pull/1758), [planet#692](https://github.com/gravitational/gravity/pull/1758), [satellite#237](https://github.com/gravitational/satellite/pull/237)).

### 7.0.10 (June 23rd, 2020)

#### Improvements

* Update `gravity status` to display a reason for degraded status ([#1722](https://github.com/gravitational/gravity/pull/1722)).

#### Bugfixes

* Fix a number of issues that could lead to blob synchronizer being stuck on removed peers ([#1648](https://github.com/gravitational/gravity/pull/1648)).
* Fix an issue with browser-based installation timing out ([#1725](https://github.com/gravitational/gravity/pull/1725)).
* Fix an issue with NFS mounts not working inside planet container ([#1743](https://github.com/gravitational/gravity/pull/1743), [planet#687](https://github.com/gravitational/planet/pull/687)).
* Fix an issue with kube-apiserver sometimes entering failed state during failover ([#1743](https://github.com/gravitational/gravity/pull/1743), [planet#689](https://github.com/gravitational/planet/pull/689)).
* Fix an issue with calculating port differences when upgrading to a new version ([#1746](https://github.com/gravitational/gravity/pull/1746)).

### 7.0.9 (June 16th, 2020)

#### Improvements

* Add backoff after new cluster image upload to make sure cluster is healthy ([#1711](https://github.com/gravitational/gravity/pull/1711)).
* Set apiserver flags to enable service account token volume projection ([#1715](https://github.com/gravitational/gravity/pull/1715), [planet#683](https://github.com/gravitational/planet/pull/683)).

### 7.0.8 (June 15th, 2020)

#### Improvements

* Implement a better default kube-apiserver audit policy ([#1693](https://github.com/gravitational/gravity/pull/1693), [planet#676](https://github.com/gravitational/planet/pull/676)).
* Disk space check will now trigger a warning on soft limit (80%) and degrade the node on hard limit (90%) ([#1693](https://github.com/gravitational/gravity/pull/1693), [planet#680](https://github.com/gravitational/planet/pull/680), [#229](https://github.com/gravitational/satellite/pull/229)).

#### Bugfixes

* Fix empty data on some default Grafana dashboards ([#1675](https://github.com/gravitational/gravity/pull/1675), [#171](https://github.com/gravitational/monitoring-app/pull/171)).

### 7.0.7 (June 9th, 2020)

#### Improvements

* Add `gravity rollback` command ([#1626](https://github.com/gravitational/gravity/pull/1626)).

#### Bugfixes

* Fix an issue with block devices/partitions propagation in planet ([#1663](https://github.com/gravitational/gravity/pull/1663), [planet#671](https://github.com/gravitational/planet/pull/671)).
* Fix an issue with `gravity resource` commands returning errors if `HOME` environment variable was not set ([#1665](https://github.com/gravitational/gravity/pull/1665)).

### 7.0.6 (June 1st, 2020)

#### Improvement

* Collect `gravity status history` as part of debug report ([#1520](https://github.com/gravitational/gravity/pull/1520))

#### Bugfixes
* Update Kubernetes to 1.17.6 (CVE-2020-8555) ([#1632](https://github.com/gravitational/gravity/pull/1487))([planet#657](https://github.com/gravitational/planet/pull/657))
* Update CNI plugins to 0.8.6 (CVE-2020-10749) ([#1616](https://github.com/gravitational/gravity/pull/1616))([planet#647](https://github.com/gravitational/planet/pull/647))
* Bump etcd to v3.4.9 ([#1617](https://github.com/gravitational/gravity/pull/1487))([planet#648](https://github.com/gravitational/planet/pull/648))
* Re-add nethealth check ([#1612](https://github.com/gravitational/gravity/pull/1612))
* Prevent shrink operation when join operation not properly initialized ([#1603](https://github.com/gravitational/gravity/pull/1603))
* Fix several issues that prevented expand operations from being resumable ([#1542](https://github.com/gravitational/gravity/pull/1542))
* Update wormhole to 0.3.2 ([#1563](https://github.com/gravitational/gravity/pull/1563))

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-announce/BGG-uvklk7Y) for more information.

!!! warning
    This release fixes a security vulnerability in CNI. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-announce/wGuDMGdnW9M) for more information.

### 7.0.5 (May 13th, 2020)

#### Improvements

* Add Teleport nodes status to `gravity status` ([#1487](https://github.com/gravitational/gravity/pull/1487)).
* Make the hook that installs a system DNS application idempotent ([#1514](https://github.com/gravitational/gravity/pull/1514)).

#### Bugfixes

* Make upgrade operation more tolerant to Kubernetes version skew policy ([#1458](https://github.com/gravitational/gravity/pull/1458)).
* Make sure Teleport node has connected successfully when joining a new node ([#1487](https://github.com/gravitational/gravity/pull/1487)).
* Fix dates displayed in the UI to use standard US format MM/DD/YYYY ([#1491](https://github.com/gravitational/gravity/pull/1491)).
* Fix an issue with inability to set a `role_map` property when configuring a trusted cluster ([#1527](https://github.com/gravitational/gravity/pull/1527)).
* Loosen preflight check to allow variance in OS patch versions between cluster nodes ([#1530](https://github.com/gravitational/gravity/pull/1530)).
* Fix an issue with bogus expand operations sometimes appearing after multi-node installs ([#1537](https://github.com/gravitational/gravity/pull/1537)).
* Fix issues with empty Prometheus metrics that prevented `kubectl top` and HPA from working ([#1543](https://github.com/gravitational/gravity/pull/1543), [monitoring-app#163](https://github.com/gravitational/monitoring-app/pull/163)).
* Update default trusted cluster role mapping to only map remote admin role to local admin role ([#1546](https://github.com/gravitational/gravity/pull/1546)).
* Fix an issue with kube-controller-manager getting unauthorized errors after changing the node advertise address ([#1548](https://github.com/gravitational/gravity/pull/1548)).
* Fix an issue with Teleport node not being able to join after changing the node advertise address ([#1548](https://github.com/gravitational/gravity/pull/1548)).

!!! warning
    This release addresses an issue with an insecure default that would map any remote role to the local admin role when connecting
    a cluster to a Hub using a trusted cluster without an explicitly configured role mapping. See [Trusted Clusters](config.md#trusted-clusters-enterprise)
    documentation for role mapping configuration details.

### 7.0.4 (April 29th, 2020)

#### Bugfixes

* Prevent nethealth checker from affecting the cluster status temporarily to avoid possible issues with cluster becoming degraded after removing a node ([#1467](https://github.com/gravitational/gravity/pull/1467)).
* Upgrade etcd to `3.3.20` in order to fix a problem with missed etcd events ([#1467](https://github.com/gravitational/gravity/pull/1467)).

### 7.0.3 (April 27th, 2020)

#### Improvements

* Update `gravity status history` to track planet leader change events ([#1449](https://github.com/gravitational/gravity/pull/1449)).
* Upgrade Helm to `v2.15.2` ([#1455](https://github.com/gravitational/gravity/pull/1455)).

#### Bugfixes

* Fix an issue with Teleport nodes failing to join after expand operation ([#1453](https://github.com/gravitational/gravity/pull/1453)).

### 7.0.2 (April 23rd, 2020)

!!! warning
    This release has a known issue that can lead to new Teleport nodes failing to join the cluster. Please use a more current release.

#### Improvements

* Update vendored Helm to version `v2.15.2` ([#1440](https://github.com/gravitational/gravity/pull/1440)).

### 7.0.1 (April 22nd, 2020)

#### Improvements

* Add overlay network checker to the in-cluster problem detector ([#1326](https://github.com/gravitational/gravity/pull/1326)).
* Add ability to view cluster operations from CLI using `gravity resource get operations` command ([#1336](https://github.com/gravitational/gravity/pull/1336)).
* Add EULA prompt to CLI installer if the application requires it ([#1379](https://github.com/gravitational/gravity/pull/1379)).

#### Bugfixes

* Fix an issue with hub certificates missing SANs after expand ([#1322](https://github.com/gravitational/gravity/pull/1322)).
* Disallow running certain commands inside planet container which could lead to unexpected results ([#1352](https://github.com/gravitational/gravity/pull/1352)).
* Fix a number of issues that could lead to expand operation being stuck ([#1361](https://github.com/gravitational/gravity/pull/1361)).
* Fix an issue with custom dashboards watcher not being able to authenticate with Grafana ([#1366](https://github.com/gravitational/gravity/pull/1366)).
* Fix an issue with wormhole CNI plugin installation ([#1370](https://github.com/gravitational/gravity/pull/1370)).
* Fix an issue with status hook not being executed to verify the application health after install and upgrade ([#1392](https://github.com/gravitational/gravity/pull/1392)).
* Fix an issue with explicitly specifying installer directory in `gravity install` command ([#1415](https://github.com/gravitational/gravity/pull/1415)).
* Update templating library to match the version used by Helm v2.15 ([#1418](https://github.com/gravitational/gravity/pull/1418)).

### 7.0.0 (April 3rd, 2020)

Gravity 7.0 is the next major release featuring improved support for existing
Kubernetes clusters, out-of-the-box integration with OpenEBS, SELinux support,
status history timeline view and a lot of other improvements and bug fixes.

Please see [Announcing Gravity 7.0](https://gravitational.com/blog/announcing-gravity-7-0/)
blog post for more details and refer to the following resources to find information
about the major new features:

* [Application Catalog](catalog.md) documentation
and a [blog post](https://gravitational.com/blog/deploying-applications-to-a-kubernetes-cluster-to-which-you-dont-have-access/)
to learn how to package Helm charts into self-contained application images and deliver
them to any Kubernetes cluster.
* [Persistent Storage](storage.md) to learn how to take advantage of the built-in OpenEBS integration.
* [SELinux](selinux.md) to learn about installing on systems with SELinux enabled.
* [Cluster Status History](cluster.md#cluster-status-history)
to learn how to gain insight into how the cluster status changes over time.

## 6.3 Releases

### 6.3.18 (June 1st, 2020)

#### Bugfixes
* Update Kubernetes to 1.17.6 (CVE-2020-8555) ([#1631](https://github.com/gravitational/gravity/pull/1487))([planet#655](https://github.com/gravitational/planet/pull/655))
* Bump etcd to v3.3.22 ([#1617](https://github.com/gravitational/gravity/pull/1487))([planet#649](https://github.com/gravitational/planet/pull/649))
* Prevent etcd portion of upgrades from taking excessive amount of time ([#1617](https://github.com/gravitational/gravity/pull/1487))
* Update CNI plugins to 0.8.6 (CVE-2020-10749) ([#1617](https://github.com/gravitational/gravity/pull/1487))([planet#649](https://github.com/gravitational/planet/pull/649))
* Fix several issues that prevented expand operations from being resumable ([#1594](https://github.com/gravitational/gravity/pull/1594))

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-announce/BGG-uvklk7Y) for more information.

!!! warning
    This release fixes a security vulnerability in CNI. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-announce/wGuDMGdnW9M) for more information.

### 6.3.17 (May 13th, 2020)

#### Bugfixes

* Update default trusted cluster role mapping to only map remote admin role to local admin role ([#1544](https://github.com/gravitational/gravity/pull/1544)).
* Fix a number of issues that may have led to crashes when trying to resume operations ([#1525](https://github.com/gravitational/gravity/pull/1525)).

!!! warning
    This release addresses an issue with an insecure default that would map any remote role to the local admin role when connecting
    a cluster to a Hub using a trusted cluster without an explicitly configured role mapping. See [Trusted Clusters](config.md#trusted-clusters-enterprise)
    documentation for role mapping configuration details.

### 6.3.16 (May 7th, 2020)

* Fix an issue with trusted cluster connection becoming invalid after gravity-site restart if trusted cluster has `role_map` defined ([#1510](https://github.com/gravitational/gravity/pull/1510)).

### 6.3.15 (May 6th, 2020)

#### Bugfixes

* Fix an issue with explicitly specifying installer directory in `gravity install` command ([#1425](https://github.com/gravitational/gravity/pull/1425)).
* Fix a number of issues that could lead to blob synchronizer being stuck on removed peers ([#1472](https://github.com/gravitational/gravity/pull/1472)).
* Loosen preflight check to allow variance in OS patch versions between cluster nodes ([#1484](https://github.com/gravitational/gravity/pull/1484)).
* Make sure `tele login` checks for cluster existence ([#1500](https://github.com/gravitational/gravity/pull/1500)).
* Fix an issue with setting `role_map` trusted cluster field ([#1504](https://github.com/gravitational/gravity/pull/1504)).

### 6.3.14 (April 29th, 2020)

#### Bugfixes

* Prevent nethealth checker from affecting the cluster status temporarily to avoid possible issues with cluster becoming degraded after removing a node ([#1466](https://github.com/gravitational/gravity/pull/1466)).
* Upgrade etcd to `3.3.20` in order to fix a problem with missed etcd events ([#1466](https://github.com/gravitational/gravity/pull/1466)).

### 6.3.13 (April 23rd, 2020)

#### Bugfixes

* Fix formatting issue in `gravity status` ([#1394](https://github.com/gravitational/gravity/pull/1394)).
* Fix an issue with cluster controller attempting to connect to the agent after operation completion ([#1420](https://github.com/gravitational/gravity/pull/1420)).
* Fix an issue with Teleport nodes failing to join after expand operation ([#1434](https://github.com/gravitational/gravity/pull/1434)).

### 6.3.12 (April 15th, 2020)

!!! warning
    This release has a known issue that can lead to new Teleport nodes failing to join the cluster. Please use a more current release.

#### Improvements

* Overlay network checker failure will now move cluster to degraded state ([#1388](https://github.com/gravitational/gravity/pull/1388)).
* Update gravity status history to track planet leader change events ([#1388](https://github.com/gravitational/gravity/pull/1388)).

#### Bugfixes

* Fix an issue with planet agent logs verbosity in case of disabled monitoring app ([#1388](https://github.com/gravitational/gravity/pull/1388)).

### 6.3.11 (April 15th, 2020)

#### Bugfixes

* Fix an issue with successfully completed join operations being marked as failed ([#1383](https://github.com/gravitational/gravity/pull/1383)).
* Fix an issue with 6.3.10 gravity binary not being able to join to older 6.3 clusters ([#1383](https://github.com/gravitational/gravity/pull/1383)).
* Fix a cosmetic issue with messages being improperly formatted when printing operations ([#1374](https://github.com/gravitational/gravity/pull/1374)).

### 6.3.10 (April 13th, 2020)

#### Improvements

* Add overlay network checker to the in-cluster problem detector ([#1324](https://github.com/gravitational/gravity/pull/1324)).
* Add ability to view cluster operations from CLI using `gravity resource get operations` command ([#1338](https://github.com/gravitational/gravity/pull/1338)).
* Set apiserver flags to enable service account token volume projection ([#1359](https://github.com/gravitational/gravity/pull/1359)).

#### Bugfixes

* Disallow running certain commands inside planet container which could lead to unexpected results ([#1350](https://github.com/gravitational/gravity/pull/1350)).
* Fix a number of issues that could lead to expand operation being stuck ([#1348](https://github.com/gravitational/gravity/pull/1348)).

### 6.3.9 (April 3rd, 2020)

#### Improvements

* Add `--pull` flag to `tele build` to allow always pulling latest versions of images ([#1309](https://github.com/gravitational/gravity/pull/1309)).

#### Bugfixes

* Apply CPU and memory limits and requests on Logrange components ([#1287](https://github.com/gravitational/gravity/pull/1287), [logging-app#64](https://github.com/gravitational/logging-app/pull/64)).
* Fix an issue with hub certificates missing SANs after expand ([#1318](https://github.com/gravitational/gravity/pull/1318)).
* Fix an issue with displaying server version in `gravity status` ([#1309](https://github.com/gravitational/gravity/pull/1309)).
* Fix a race condition that could lead to planet rootfs being reset during upgrade ([#1309](https://github.com/gravitational/gravity/pull/1309)).

### 6.3.8 (March 23rd, 2020)

#### Bugfixes

* Fix an issue with missing helm symlink on reinstall ([#1103](https://github.com/gravitational/gravity/pull/1103)).
* Fix an issue with dns configuration during upgrade when host DNS has a localhost resolver ([#1161](https://github.com/gravitational/gravity/pull/1161)).
* Fix an issue where the main upgrade agent could begin upgrade steps before all node agents are available ([#1205](https://github.com/gravitational/gravity/pull/1205)).
* Agent deployment will now retry on transient network errors ([#1205](https://github.com/gravitational/gravity/pull/1205)).
* If the upgrade fails to initialize, shut down all upgrade agents ([#1205](https://github.com/gravitational/gravity/pull/1205)).
* Fix an issue with uploading teleport session logs ([#1225](https://github.com/gravitational/gravity/pull/1225)).
* Prevent grafana from attempting to contact analytics servers ([#1250](https://github.com/gravitational/gravity/pull/1250)).
* Fix an issue with serf members not leaving the cluster ([#1260](https://github.com/gravitational/gravity/pull/1260)).
* Upgrade Kubernetes to `v1.17.04` (CVE-2020-8551, CVE-2020-8552) ([#1271](https://github.com/gravitational/gravity/pull/1271)).

#### Improvements

* Implement `gravity status history` command to show status changes ([#1119](https://github.com/gravitational/gravity/pull/1119)).
* `gravity status` now shows both the client and server version in the status output ([#1166](https://github.com/gravitational/gravity/pull/1166)).
* The runtime will now properly check and prevent upgrades on unsupported upgrade paths ([#1237](https://github.com/gravitational/gravity/pull/1237)).
* Add cgroup cleaner to planet to prevent leaking cgroups ([planet#576](https://github.com/gravitational/planet/pull/576)).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-announce/jPiyJ1KL_FI) for more information.

### 6.3.7 (February 12th, 2020)

#### Improvements

* Update Kubernetes to `v1.17.2` ([#1080](https://github.com/gravitational/gravity/pull/1080)).

#### Bugfixes

* Fix an issue with merging `ClusterConfiguration` resource and validation checks ([#1093](https://github.com/gravitational/gravity/pull/1093)).
* Update kernel module checker to support 5.0/5.1 Linux kernels ([#1094](https://github.com/gravitational/gravity/pull/1094)).

### 6.3.6 (February 4th, 2020)

#### Bugfixes

* Fix an issue with flannel incorrectly recovering from a watch failure ([#1070](https://github.com/gravitational/gravity/pull/1070)).
* Fix an issue with changes to pod CIDR within cluster configuration ([#1043](https://github.com/gravitational/gravity/pull/1043)).
* Fix broken menu height and scrollbars ([#1042](https://github.com/gravitational/gravity/pull/1042)).
* Fix a UI issue with null items returned by kubernetes API ([#1039](https://github.com/gravitational/gravity/pull/1039)).
* Enable all kubernetes default admission controllers ([#1069](https://github.com/gravitational/gravity/pull/1069)).

### 6.3.5 (January 16th, 2020)

#### Bugfixes

* Fix the issue with gravity-site sometimes failing to start with bad permissions error ([#1024](https://github.com/gravitational/gravity/pull/1024)).

### 6.3.4 (January 14th, 2020)

#### Improvements

* Add Amazon Linux 2 to supported distros of the base Gravity image ([#1019](https://github.com/gravitational/gravity/pull/1019)).

#### Bugfixes

* Fix the issue with "role not found" error when trying to access remote clusters via Gravity Hub ([#1012](https://github.com/gravitational/gravity/pull/1012)).

### 6.3.3 (January 8th, 2020)

#### Bugfixes

* Restore automatic node registration via kubelet ([#1001](https://github.com/gravitational/gravity/pull/1001), [planet#539](https://github.com/gravitational/planet/pull/539)).

### 6.3.2 (December 20th, 2019)

#### Bugfixes

* Fix an issue with CoreDNS pods not being scheduled due to discrepancy between node selector and node labels ([#985](https://github.com/gravitational/gravity/pull/985)).

### 6.3.1 (December 20th, 2019)

#### Bugfixes

* Fix a security issue where secrets are being reused for multiple certificates and secrets are not being rotated during certificate rotation [#979](https://github.com/gravitational/gravity/pull/979).

!!! warning
    This release fixes a security vulnerability in gravity.

### 6.3.0 (December 18th, 2019)

#### Improvements

* Upgrade Kubernetes to `v1.17.0` ([#967](https://github.com/gravitational/gravity/pull/967), [planet#537](https://github.com/gravitational/planet/pull/537)).
* Remove Docker bridge and promiscuous mode configurations ([#959](https://github.com/gravitational/gravity/pull/959), [planet#536](https://github.com/gravitational/planet/pull/536)).
* Use relative binary path when displaying `gravity join` command hint ([#935](https://github.com/gravitational/gravity/pull/935)).

#### Bugfixes

* Fix the issue with kubelet failing to start with unsupported labels ([#953](https://github.com/gravitational/gravity/pull/953)).
* Fix the issue with `gravity status` becoming slow when there are a lot of namespaces ([#956](https://github.com/gravitational/gravity/pull/956)).
* Fix the issue with disconnecting clusters from the Hub ([#964](https://github.com/gravitational/gravity/pull/964)).

## 6.2 Releases

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

### 6.2.2 (October 17th, 2019)

#### Bugfixes

* Upgrade Kubernetes to `v1.16.02` (CVE-2019-11253) ([#808](https://github.com/gravitational/gravity/pull/808)).

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

### 6.2.0 (September 24th, 2019)

#### Improvements

* Upgrade Kubernetes to `v1.16.0`.

## 6.1 Releases

### 6.1.48 LTS (March 23, 2021)

#### Bugfixes
* Fix coredns-worker podAntiAffinity to correctly select on kube-dns-worker. [#2389](https://github.com/gravitational/gravity/pull/2389)

#### Improvements
* Add a feature to planet to hardcode the google metadata server into the planet hosts file when detected as running on a gcloud VM. [#2441](https://github.com/gravitational/gravity/pull/2441), [planet#817](https://github.com/gravitational/planet/pull/817)
* Add AliasIP range support to flannel. [#2441](https://github.com/gravitational/gravity/pull/2441), [planet#832](https://github.com/gravitational/planet/pull/832), [flannel#10](https://github.com/gravitational/flannel/pull/10)

### 6.1.47 LTS (January 9, 2021)

#### Bugfixes
* Fix an issue with file descriptors leaking in monitoring network health ([#2382](https://github.com/gravitational/gravity/pull/2382), [monitoring-app#205](https://github.com/gravitational/monitoring-app/pull/205), [satellite#293](https://github.com/gravitational/satellite/pull/293)).

### 6.1.46 LTS (December 7, 2020)

#### Improvements
* Add support for using a separate network project in GCE to flannel ([#2356](https://github.com/gravitational/gravity/pull/2356), [planet#808](https://github.com/gravitational/planet/pull/808), [flannel#9](https://github.com/gravitational/satellite/pull/9)).
* Remove CONFIG_NF_NAT_IPv4 check to support RHEL 8.3 based kernels ([#2337](https://github.com/gravitational/gravity/pull/2337), [planet#797](https://github.com/gravitational/planet/pull/797), [satellite#288](https://github.com/gravitational/satellite/pull/288)).

#### Bugfixes
* Shutdown kubernetes control plane immediately when elections are disabled ([#2356](https://github.com/gravitational/gravity/pull/2356), [planet#804](https://github.com/gravitational/planet/pull/804)).
* Increase the amount of time dns-app update hooks will wait for cluster changes to complete ([#2327](https://github.com/gravitational/gravity/pull/2327)).
* Increase file descriptor limits when creating systemd units ([#2322](https://github.com/gravitational/gravity/pull/2322)).

### 6.1.45 LTS (November 5th, 2020)

#### Improvements

* Allow configuration of the Pod subnet size through the Cluster Configuration resource ([#2305](https://github.com/gravitational/gravity/pull/2305), [planet#788](https://github.com/gravitational/planet/pull/788)).


### 6.1.44 LTS (October 22nd, 2020)

#### Improvements

* Update TLS cipher suites for Kubernetes components ([#2264](https://github.com/gravitational/gravity/pull/2264), [planet#781](https://github.com/gravitational/planet/pull/781)).
* Satellite queries for system pods will use less load by searching only the `kube-system` and `monitoring` namespaces for critical pods that aren't running ([#2249](https://github.com/gravitational/gravity/pull/2249), [planet#774](https://github.com/gravitational/planet/pull/774), [satellite#280](https://github.com/gravitational/satellite/pull/280)).

#### Bugfixes

* Fix an issue where flannel could corrupt iptables rules if newly generated rules don't exactly match rules previously used ([#2264](https://github.com/gravitational/gravity/pull/2264), [planet#778](https://github.com/gravitational/planet/pull/778), [flannel#7](https://github.com/gravitational/flannel/pull/7)).
* Fix an issue when using GCE integrations that unnecessary OAuth scopes would be requested ([#2264](https://github.com/gravitational/gravity/pull/2264), [planet#778](https://github.com/gravitational/planet/pull/778), [flannel#7](https://github.com/gravitational/flannel/pull/8)).
* Fix an issue where etcd-backups were using too short of a timer ([#2250](https://github.com/gravitational/gravity/pull/2250), [planet#768](https://github.com/gravitational/planet/pull/768), [etcd-backup#5](https://github.com/gravitational/satellite/pull/5)).
* Fix an issue where cluster configuration could be lost during validation ([#2256](https://github.com/gravitational/gravity/pull/2256)).
* Fix an issue that prevents log truncation ([#2237](https://github.com/gravitational/gravity/pull/2237), [logging-app#72](https://github.com/gravitational/logging-app/pull/72)).

### 6.1.43 LTS (October 13th, 2020)

#### Improvements
* Tune gravity to support larger clusters ([#2220](https://github.com/gravitational/gravity/pull/2220)).
* Add check that phases are rolled back in the correct order ([#2218](https://github.com/gravitational/gravity/pull/2218)).

### 6.1.42 LTS (October 6th, 2020)

#### Improvements
* GRPC logging will be enabled when passing the `--debug` flag to gravity commands ([#2178](https://github.com/gravitational/gravity/pull/2178)).

#### Bugfixes

* Fix a security issue when using SAML authentication to an identity provider (CVE-2020-15216) ([#2193](https://github.com/gravitational/gravity/pull/2193)).
* Fix an issue with serf clusters missing nodes when partitioned for more than 24 hours ([#2176](https://github.com/gravitational/gravity/pull/2176), [planet#759](https://github.com/gravitational/planet/pull/759)).

!!! warning
    This release fixes a security vulnerability in Teleport when connecting Gravity to a SAML 2.0 identity provider. Please see
    [Teleport Announcement for CVE-2020-15216](https://github.com/gravitational/teleport/releases/tag/v4.3.7) for more information.

### 6.1.41 LTS (October 2nd, 2020)

#### Improvements

* Improve operation plan sync resiliency in case of temporary etcd issues ([#2144](https://github.com/gravitational/gravity/pull/2144)).
* Gravity debug report will now include Gravity resources ([#2161](https://github.com/gravitational/gravity/pull/2161)).
* Update a minimum required subnet size to be a /22 ([#2183](https://github.com/gravitational/gravity/pull/2183)).

#### Bugfixes

* Fix an issue with inability to resume operation when gravity-site or etcd are down ([#2165](https://github.com/gravitational/gravity/pull/2165)).
* Fix an issue with preflight checks not taking mount overrides into account ([#2168](https://github.com/gravitational/gravity/pull/2168)).

### 6.1.40 LTS (September 25th, 2020)

#### Improvements

* Degrade cluster status if any of the nodes are offline ([#2130](https://github.com/gravitational/gravity/pull/2130)).
* Make sure upgrade agents are online when resuming or rolling back the operation ([#2071](https://github.com/gravitational/gravity/pull/2071)).
* Scale Prometheus/Alertmanager replicas according to the number of cluster nodes ([#2145](https://github.com/gravitational/gravity/pull/2145), [monitoring#188](https://github.com/gravitational/monitoring-app/pull/188)).

#### Bugfixes

* Fix an issue with installer intermittently failing with the "connection refused" error ([#2151](https://github.com/gravitational/gravity/pull/2151)).
* Fix an issue with wizard installation ([#2135](https://github.com/gravitational/gravity/pull/2135)).

### 6.1.39 LTS (September 15th, 2020)

#### Bugfixes

* Fix a regression in `tele build` when tele would fail to pull the dependent planet package ([#2126](https://github.com/gravitational/gravity/pull/2126)).

### 6.1.38 LTS (September 11th, 2020)

#### Improvements

* Increase default `LimitNOFile` parameter within planet's systemd to `655350` ([#2092](https://github.com/gravitational/gravity/pull/2092), [planet#743](https://github.com/gravitational/planet/pull/743)).
* Upgrade Helm to `v2.15.2` ([#2118](https://github.com/gravitational/gravity/pull/2118), [planet#747](https://github.com/gravitational/planet/pull/747)).
* Shrink operation resiliency improvements ([#2096](https://github.com/gravitational/gravity/issues/2096)).

#### Bugfixes

* Fix an issue with etcd not being upgraded on the worker nodes ([#2097](https://github.com/gravitational/gravity/issues/2097), [planet#740](https://github.com/gravitational/planet/pull/740)).
* Fix an issue with install not being able to uninstall existing failed planet service ([#2110](https://github.com/gravitational/gravity/issues/2110)).
* Fix an issue with upgrade when custom planet container is used ([#2094](https://github.com/gravitational/gravity/issues/2094)).
* Fix an issue with CoreDNS high CPU usage when configured with non-existing upstream server ([planet#745](https://github.com/gravitational/planet/pull/745)).

### 6.1.37 LTS (August 31st, 2020)

#### Improvements

* Add checker that validates Gravity data directory user/group ownership ([#2057](https://github.com/gravitational/gravity/pull/2057)).
* Validate `HTTP_PROXY` and `HTTPS_PROXY` variables set via `RuntimeEnvironment` resource ([#2061](https://github.com/gravitational/gravity/pull/2061)).

#### Bugfixes

* Fix an issue with installation sometimes failing when trying to install a cluster license ([#2054](https://github.com/gravitational/gravity/pull/2054)).

### 6.1.36 LTS (August 20th, 2020)

#### Bugfixes

* Fix an issue with upgrading applications that use custom planet containers that contained corrupted version metadata ([#2032](https://github.com/gravitational/gravity/pull/2032)).
* Fix an issue with upgrades if etcd is deployed within kubernetes and scheduled to a master ([#2024](https://github.com/gravitational/gravity/pull/2024), [planet#721](https://github.com/gravitational/planet/pull/721)).
* Fix an issue with corrupted version metadata within custom planet containers ([#2024](https://github.com/gravitational/gravity/pull/2024), [planet#717](https://github.com/gravitational/planet/pull/717)).

### 6.1.35 LTS (August 17th, 2020)

#### Bugfixes

* Fix an issue with building cluster images that use custom planet containers ([#2015](https://github.com/gravitational/gravity/pull/2015)).

### 6.1.34 LTS (August 14th, 2020)

#### Improvements

* Upgrade agents will now restart automatically if nodes reboot during upgrade ([#1952](https://github.com/gravitational/gravity/pull/1952)).
* Add ability to display deployed agent status using `gravity agent status` command ([#1952](https://github.com/gravitational/gravity/pull/1952)).
* Update `gravity plan resume` command to launch from a systemd service by default, `--block` flag is supported for old behavior ([#1935](https://github.com/gravitational/gravity/pull/1935)).
* Add ability to follow operation plan using `gravity plan --tail` command ([#1935](https://github.com/gravitational/gravity/pull/1935)).

#### Bugfixes

* Fixes an issue with upgrades when using alternate install location ([#2010](https://github.com/gravitational/gravity/pull/2010)).
* Fix an issue with log forwarding configuration breaking during upgrade ([#1973](https://github.com/gravitational/gravity/pull/1973), [logging-app#68](https://github.com/gravitational/planet/pull/68)).
* Fix several issues with using custom planet containers ([#1962](https://github.com/gravitational/gravity/pull/1962)).

### 6.1.33 LTS (July 30th, 2020)

#### Improvements

* Remove the hard limit of 3 master nodes. ([#1910](https://github.com/gravitational/gravity/pull/1910)).
* Improved warnings when teleport nodes are using the wrong join token ([#1902](https://github.com/gravitational/gravity/pull/1902)).
* Collect gravity cli history for debug reports ([#1860](https://github.com/gravitational/gravity/pull/1860)).

#### Bugfixes

* Update CoreDNS to 1.7.0 ([#1924](https://github.com/gravitational/gravity/pull/1924), [planet#702](https://github.com/gravitational/planet/pull/702)).
* Update ContainerD to 1.2.10 ([#1924](https://github.com/gravitational/gravity/pull/1924), [planet#710](https://github.com/gravitational/planet/pull/710)).
* Disable CSIMigration feature gate ([#1924](https://github.com/gravitational/gravity/pull/1924), [planet#709](https://github.com/gravitational/planet/pull/709)).

### 6.1.31 LTS (July 13th, 2020)

#### Bugfixes

* Fix a regression in `6.1.30` that led to installation crashing in bare-metal mode during package configuration phase ([#1864](https://github.com/gravitational/gravity/pull/1864)).

### 6.1.30 LTS (July 10th, 2020)

#### Improvements

* Disk space check will now trigger a warning on soft limit (80%) and degrade the node on hard limit (90%) ([#1690](https://github.com/gravitational/gravity/pull/1690), [planet#678](https://github.com/gravitational/planet/pull/678), [#228](https://github.com/gravitational/satellite/pull/228)).
* Add backoff after new cluster image upload to make sure cluster is healthy ([#1709](https://github.com/gravitational/gravity/pull/1709)).
* Display a more detailed reason for degraded status in `gravity status` ([#1723](https://github.com/gravitational/gravity/pull/1723)).
* Update `gravity status` to display unhealthy critical system pods ([#1752](https://github.com/gravitational/gravity/pull/1752), [planet#690](https://github.com/gravitational/planet/pull/690), [satellite#235](https://github.com/gravitational/satellite/pull/235)).
* Upgrade Grafana to `v6.7.4` ([#1754](https://github.com/gravitational/gravity/pull/1754), [monitoring#177](https://github.com/gravitational/monitoring-app/pull/177)).
* Add `--since` flag to `gravity report` command to allow filtering collected logs by time ([#1800](https://github.com/gravitational/gravity/pull/1800)).
* Add more system information collectors to `gravity report` ([#1800](https://github.com/gravitational/gravity/pull/1800)).
* Update `ClusterConfiguration` resource to support service CIDR changes ([#1839](https://github.com/gravitational/gravity/pull/1839)).

#### Bugfixes

* Fix an issue with monitoring app upgrade sometimes failing with "request too large" ([#1697](https://github.com/gravitational/gravity/pull/1697), [monitoring#174](https://github.com/gravitational/monitoring-app/pull/174)).
* Fix an issue with `kube-apiserver` sometimes entering failed state during failover ([#1742](https://github.com/gravitational/gravity/pull/1742), [planet#685](https://github.com/gravitational/planet/pull/685)).
* Fix an issue with calculating port differences when upgrading to a new version ([#1745](https://github.com/gravitational/gravity/pull/1745)).
* Fix an issue with RPC agent credentials not being rotated during upgrade ([#1747](https://github.com/gravitational/gravity/pull/1747)).
* Make sure upgrade is resumed on the correct node to avoid issues with plan inconsistency ([#1793](https://github.com/gravitational/gravity/pull/1793)).
* Make sure upgrade can't be launched if the previous upgrade wasn't completed or fully rolled back ([#1802](https://github.com/gravitational/gravity/pull/1802)).
* Fix an issue with upgrade failing to adjust volume permissions if only uid is specified ([#1802](https://github.com/gravitational/gravity/pull/1802)).

### 6.1.29 LTS (June 12th, 2020)

#### Bugfixes

* Fix an issue with upgrading from releases containing etcd `v3.3.20` ([#1694](https://github.com/gravitational/gravity/pull/1694), [#679](https://github.com/gravitational/planet/pull/679)).
* Fix an issue with nethealth checker not removing metrics for removed nodes ([#1621](https://github.com/gravitational/gravity/pull/1621), [#638](https://github.com/gravitational/planet/pull/638), [monitoring-app#167](https://github.com/gravitational/monitoring-app/pull/167)).

### 6.1.28 LTS (June 1st, 2020)

#### Improvements

* Add gravity rollback command ([#1620](https://github.com/gravitational/gravity/pull/1620))
* Add planet election change events to `gravity status history` ([#1355](https://github.com/gravitational/gravity/pull/1355))

#### Bugfixes
* Update Kubernetes to 1.15.12 (CVE-2020-8555) ([#1627](https://github.com/gravitational/gravity/pull/1487))([planet#656](https://github.com/gravitational/planet/pull/656))
* Bump etcd to v3.3.22 ([#1618](https://github.com/gravitational/gravity/pull/1487))([planet#648](https://github.com/gravitational/planet/pull/648))
* Prevent etcd portion of upgrades from taking excessive amount of time ([#1618](https://github.com/gravitational/gravity/pull/1487))
* Update CNI plugins to 0.8.6 (CVE-2020-10749) ([#1618](https://github.com/gravitational/gravity/pull/1487))([planet#648](https://github.com/gravitational/planet/pull/648))
* Fix several issues that prevented expand operations from being resumable ([#1605](https://github.com/gravitational/gravity/pull/1605))
* Use consistent naming to prevent issues with re-running phases ([#1601](https://github.com/gravitational/gravity/pull/1601))

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-announce/BGG-uvklk7Y) for more information.

!!! warning
    This release fixes a security vulnerability in CNI. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-announce/wGuDMGdnW9M) for more information.

### 6.1.27 LTS (May 13th, 2020)

#### Bugfixes

* Fix an issue with inability to set a `role_map` property when configuring a trusted cluster ([#1557](https://github.com/gravitational/gravity/pull/1557)).
* Update default trusted cluster role mapping to only map remote admin role to local admin role ([#1557](https://github.com/gravitational/gravity/pull/1557)).
* Fix an issue with wormhole CNI plugin installation ([#1565](https://github.com/gravitational/gravity/pull/1565)).

!!! warning
    This release addresses an issue with an insecure default that would map any remote role to the local admin role when connecting
    a cluster to a Hub using a trusted cluster without an explicitly configured role mapping. See [Trusted Clusters](config.md#trusted-clusters-enterprise)
    documentation for role mapping configuration details.

### 6.1.26 LTS (May 13th, 2020)

#### Improvements

* Add ability to display warning probes in `gravity status` ([#1499](https://github.com/gravitational/gravity/pull/1499)).
* Add Teleport nodes status to `gravity status` ([#1486](https://github.com/gravitational/gravity/pull/1486)).
* Make the hook that installs the system DNS application idempotent ([#1513](https://github.com/gravitational/gravity/pull/1513)).

#### Bugfixes

* Make sure Teleport node has connected successfully when joining a new node ([#1486](https://github.com/gravitational/gravity/pull/1486)).
* Loosen preflight check to allow variance in OS patch versions between cluster nodes ([#1551](https://github.com/gravitational/gravity/pull/1551)).
* Fix an issue with wormhole failing to upgrade due to file permission errors ([#1562](https://github.com/gravitational/gravity/pull/1562)).

### 6.1.25 LTS (April 29th, 2020)

#### Bugfixes

* Prevent nethealth checker from affecting the cluster status temporarily to avoid possible issues with cluster becoming degraded after removing a node ([#1465](https://github.com/gravitational/gravity/pull/1465)).
* Upgrade etcd to `3.3.20` in order to fix a problem with missed etcd events ([#1465](https://github.com/gravitational/gravity/pull/1465)).

### 6.1.24 LTS (April 27th, 2020)

#### Bugfixes

* Fix an issue with wormhole image upgrade hook ([#1457](https://github.com/gravitational/gravity/pull/1457))
* Fix an issue with the join token ([#1444](https://github.com/gravitational/gravity/pull/1444)).
* Fix the upgrade to adhere to the Kubernetes version skew ([#1459](https://github.com/gravitational/gravity/pull/1459)).

#### Improvements

* Add CentOS 8 to the list of supported distributions ([#1412](https://github.com/gravitational/gravity/pull/1412)).
* Invoke status hook after successful installation/upgrade ([#1385](https://github.com/gravitational/gravity/pull/1385))
* Add EULA prompt to CLI installer if the application requires it ([#1375](https://github.com/gravitational/gravity/pull/1375)).

### 6.1.22 LTS (April 14th, 2020)

#### Bugfixes

* Fix an issue with custom dashboards watcher not being able to authenticate with Grafana ([#1364](https://github.com/gravitational/gravity/pull/1364)).
* Fix an issue with wormhole CNI plugin installation ([#1371](https://github.com/gravitational/gravity/pull/1371)).

### 6.1.21 LTS (April 10th, 2020)

#### Improvements

* Add overlay network checker to the in-cluster problem detector ([#1321](https://github.com/gravitational/gravity/pull/1321)).
* Add ability to view cluster operations from CLI using `gravity resource get operations` command ([#1337](https://github.com/gravitational/gravity/pull/1337)).
* Implement planet container changes allowing easier integration with OpenEBS ([#1344](https://github.com/gravitational/gravity/pull/1344)).

#### Bugfixes

* Disallow running certain commands inside planet container which could lead to unexpected results ([#1351](https://github.com/gravitational/gravity/pull/1351)).

### 6.1.20 LTS (March 31st, 2020)

#### Improvements

* Add `--pull` flag to `tele build` to allow always pulling latest versions of images ([#1302](https://github.com/gravitational/gravity/pull/1302)).

#### Bugfixes

* Apply CPU and memory limits and requests on Logrange components ([#1286](https://github.com/gravitational/gravity/pull/1286), [logging-app#64](https://github.com/gravitational/logging-app/pull/64)).
* Fix an issue with displaying server version in `gravity status` ([#1306](https://github.com/gravitational/gravity/pull/1306)).
* Fix a race condition that could lead to planet rootfs being reset during upgrade ([#1306](https://github.com/gravitational/gravity/pull/1306)).

### 6.1.19 LTS (March 23rd, 2020)

#### Bugfixes

* Fix an issue with serf members not leaving the cluster ([#1251](https://github.com/gravitational/gravity/pull/1251)).
* Fix an issue with changing the pod CIDR on a running cluster ([#1045](https://github.com/gravitational/gravity/pull/1045)).
* Prevent grafana from attempting to contact analytics servers ([#1250](https://github.com/gravitational/gravity/pull/1249)).
* Fix an issue with uploading teleport session logs ([#1224](https://github.com/gravitational/gravity/pull/1224)).
* Fix an issue where the main upgrade agent could begin upgrade steps before all node agents are available ([#1204](https://github.com/gravitational/gravity/pull/1204)).
* Agent deployment will now retry on transient network errors ([#1204](https://github.com/gravitational/gravity/pull/1204)).
* If the upgrade fails to initialize, shut down all upgrade agents ([#1204](https://github.com/gravitational/gravity/pull/1204)).
* Fix an issue with dns configuration during upgrade when host DNS has a localhost resolver ([#1162](https://github.com/gravitational/gravity/pull/1162)).
* Upgrade Kubernetes to `v1.15.11` (CVE-2020-8551, CVE-2020-8552) ([#1272](https://github.com/gravitational/gravity/pull/1272)).

#### Improvements

* Implement more extensive validations on ClusterConfiguration resource ([#1045](https://github.com/gravitational/gravity/pull/1045)).
* The runtime will now properly check and prevent upgrades on unsupported upgrade paths ([#1236](https://github.com/gravitational/gravity/pull/1236)).
* `gravity status` now shows both the client and server version in the status output ([#1165](https://github.com/gravitational/gravity/pull/1166)).
* Implement `gravity status history` command to show status changes ([#1118](https://github.com/gravitational/gravity/pull/1118)).
* Implement some low level convenience commands `gravity system service [stop/start/journal]` ([1104](https://github.com/gravitational/gravity/pull/1104)).
* Add cgroup cleaner to planet to prevent leaking cgroups ([planet#577](https://github.com/gravitational/planet/pull/577)).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-announce/jPiyJ1KL_FI) for more information.

### 6.1.18 LTS (February 4th, 2020)

#### Improvements

* Make username/password for SMTP configuration optional ([#1062](https://github.com/gravitational/gravity/pull/1062)).

#### Bugfixes

* Fix an issue with flannel incorrectly recovering from a watch failure ([#1070](https://github.com/gravitational/gravity/pull/1070)).
* Enable all kubernetes default admission controllers ([#1070](https://github.com/gravitational/gravity/pull/1070)).

### 6.1.17 LTS (January 27th, 2020)

#### Improvements

* Improve support for bootable configuration checker on newer kernels ([#1033](https://github.com/gravitational/gravity/pull/1033))

#### Bugfixes

* Fix an issue in the WebUI ([#1037](https://github.com/gravitational/gravity/pull/1037))
* Fix a broken helm symlink ([#1033](https://github.com/gravitational/gravity/pull/1033))

### 6.1.16 LTS (January 14th, 2020)

#### Improvements

* Add Amazon Linux 2 to supported distros of the base Gravity image ([#1018](https://github.com/gravitational/gravity/pull/1018)).

#### Bugfixes

* Fix the issue with Gravity Hub installation ([#994](https://github.com/gravitational/gravity/pull/994)).
* Restore automatic node registration via kubelet ([#1014](https://github.com/gravitational/gravity/pull/1014), [planet#541](https://github.com/gravitational/planet/pull/541)).
* Fix the issue with "role not found" error when trying to access remote clusters via Gravity Hub ([#1010](https://github.com/gravitational/gravity/pull/1010)).

### 6.1.15 LTS (December 20th, 2019)

#### Bugfixes

* Fix an issue with CoreDNS pods not being scheduled due to discrepancy between node selector and node labels ([#986](https://github.com/gravitational/gravity/pull/986)).

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

### 6.1.13 LTS (November 20th, 2019)

#### Improvements

* Display cluster name on a separate line in `gravity status` ([#895](https://github.com/gravitational/gravity/pull/895)).

#### Bugfixes

* Fix the issue with `gravity leave --force` leaving the node in partially cleaned up state when run without `sudo` ([#895](https://github.com/gravitational/gravity/pull/895)).
* Fix the issue with joining agent connecting to the ongoing installation instead of waiting for install to complete ([#885](https://github.com/gravitational/gravity/pull/885)).

### 6.1.12 LTS (November 11th, 2019)

#### Improvements

* Execute preflight checks during join operation ([#853](https://github.com/gravitational/gravity/pull/853)).
* Update `gravity check` command to support upgrade preflight checks ([#870](https://github.com/gravitational/gravity/pull/870)).
* Disable unused Docker bridge interface ([#873](https://github.com/gravitational/gravity/pull/873),  [planet#527](https://github.com/gravitational/planet/pull/527)).

### 6.1.11 LTS (October 31st, 2019)

#### Bugfixes

* Fix the issue with cluster erroneously returning to the active state after upgrade operation failure ([#846](https://github.com/gravitational/gravity/pull/846), [#857](https://github.com/gravitational/gravity/pull/857)).

### 6.1.10 LTS (October 24th, 2019)

#### Bugfixes

* Fix the issue with the installer systemd unit failing due to long command when installing with a `--license` flag ([#821](https://github.com/gravitational/gravity/pull/821)).
* Fix the issue with application-only (without runtime) upgrades ([#835](https://github.com/gravitational/gravity/pull/835)).

### 6.1.9 LTS (October 21st, 2019)

#### Bugfixes

* Fix the issue with accessing remote clusters via a Hub using `tsh` or web terminal ([#814](https://github.com/gravitational/gravity/pull/814)).
* Fix the issue with `tiller` server not being upgraded when upgrading from early 6.1 releases ([#818](https://github.com/gravitational/gravity/pull/818)).

### 6.1.8 LTS (October 17th, 2019)

#### Bugfixes

* Upgrade Kubernetes to `v1.15.05` (CVE-2019-11253) ([#809](https://github.com/gravitational/gravity/pull/809)).
* Fix an issue with upgrades related to fetching legacy teleport configuration ([#809](https://github.com/gravitational/gravity/pull/809)).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://github.com/kubernetes/kubernetes/issues/83253) for more information.

### 6.1.7 LTS (October 11th, 2019)

#### Bugfixes

* Fix the issue with join failing with "bad username or password" when using auto-scaling groups on AWS ([#789](https://github.com/gravitational/gravity/pull/789)).
* Fix the issue with web UI installer displaying the login screen ([#793](https://github.com/gravitational/gravity/pull/793)).
* Fix the issue with UI showing "user not found" error after choosing a password for a new user ([#793](https://github.com/gravitational/gravity/pull/793)).

### 6.1.6 LTS (October 10th, 2019)

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

### 6.1.5 LTS (September 18th, 2019)

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

### 6.1.4 LTS (September 10th, 2019)

#### Bugfixes

* Fix `allowPrivileged` flag placement in the manifest schema.

### 6.1.3 LTS (September 9th, 2019)

#### Improvements

* Add ability to run privileged containers. See [Running Privileged Containers](faq.md#running-privileged-containers) for details.

### 6.1.2 LTS (August 26th, 2019)

#### Bugfixes

* Upgrade golang to `v1.12.9` (CVE-2019-9512, CVE-2019-9514)
* Upgrade Kubernetes to `v1.15.2` (CVE-2019-9512, CVE-2019-9514).

!!! warning
    This release fixes a security vulnerability in golang used by gravity and kubernetes. Please see
    [Netflix Announcement](https://github.com/Netflix/security-bulletins/blob/master/advisories/third-party/2019-002.md) for more information.

### 6.1.1 LTS (August 6th, 2019)

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

### 6.1.0 LTS (August 2nd, 2019)

#### Improvements

* Upgrade Kubernetes to `1.15.1`.

## 6.0 Releases

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

### 6.0.7 LTS (September 10th, 2019)

#### Bugfixes

* Fix `allowPrivileged` flag placement in the manifest schema.

### 6.0.6 (September 9th, 2019)

#### Improvements

* Add ability to run privileged containers. See [Running Privileged Containers](faq.md#running-privileged-containers) for details.

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

## 5.6 Releases

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

### 5.6.7 (August 26th, 2019)

#### Bugfixes

* Upgrade golang to `v1.12.9` (CVE-2019-9512, CVE-2019-9514)
* Upgrade Kubernetes to `v1.14.6` (CVE-2019-9512, CVE-2019-9514).

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

### 5.6.5 (July 18th, 2019)

#### Bugfixes

* Workaround for installation failures when populating the docker registry.

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

### 5.6.3

#### Improvements

* Upgrade Kubernetes to `v1.14.2` (CVE-2019-1002101)

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/zqklrWzeA2c) for more information.

### 5.6.2

#### Improvements

* Add support for using `helm` directly from host.

### 5.6.1

#### Improvements

* Upgrade Docker to `18.09.5`.

### 5.6.0

#### Improvements

* Upgrade Kubernetes to `v1.14.0`.

## 5.5 Releases

### 5.5.59 LTS (March 30, 2021)
#### Bugfixes
* Fix WebUI metrics retention page sometimes failing to load after reboot ([#2431](https://github.com/gravitational/gravity/pull/2431)).

### 5.5.58 LTS (February 4, 2021)
#### Improvements
* Include host record for google metadata server ([#2408](https://github.com/gravitational/gravity/pull/2408), [planet#820](https://github.com/gravitational/planet/pull/820)).
* Disable Kapacitor alerts from gravity status and introduce hidden command ([#2388](https://github.com/gravitational/gravity/pull/2388)).

### 5.5.57 LTS (January 15, 2021)
#### Improvements
* Rollback of dns-app, monitoring-app, gravity-site are now re-entrant ([#2390](https://github.com/gravitational/gravity/pull/2390), [rigging#87](https://github.com/gravitational/rigging/pull/87)).
* Update of dns-app, monitoring-app, gravity-site are now re-entrant ([#2390](https://github.com/gravitational/gravity/pull/2390), [rigging#86](https://github.com/gravitational/rigging/pull/86)).


#### Bugfixes
* Shutdown kubernetes control plane immediately when elections are disabled ([#2388](https://github.com/gravitational/gravity/pull/2388), [planet#803](https://github.com/gravitational/planet/pull/803)).


### 5.5.56 LTS (November 23, 2020)

#### Improvements
* Add support for Redhat/Centos 7.9 and 8.3 ([#2338](https://github.com/gravitational/gravity/pull/2335), [planet#798](https://github.com/gravitational/planet/pull/798), [satellite#287](https://github.com/gravitational/satellite/pull/287)).

#### Bugfixes

* Fix an issue with etcd upgrades breaking watches in planet services ([#2330](https://github.com/gravitational/gravity/pull/2330)).
* Fix an issue where monitoring-app would not tolerate customer applied taints ([#2341](https://github.com/gravitational/gravity/pull/2341), [monitoring-app#200](https://github.com/gravitational/monitoring-app/pull/200)).
* Fix an issue where logging-app would not tolerate customer applied taints ([#2341](https://github.com/gravitational/gravity/pull/2341), [logging-app#73](https://github.com/gravitational/logging-app/pull/73)).
* Increase the amount of time dns-app update hooks will wait for cluster changes to complete ([#2328](https://github.com/gravitational/gravity/pull/2328)).
* Increase FD limits when creating systemd units ([#2315](https://github.com/gravitational/gravity/pull/2315)).

### 5.5.55 LTS (October 21st, 2020)

#### Improvements

* Update system pods checker to query only specific namespaces ([#2248](https://github.com/gravitational/gravity/pull/2248), [planet#773](https://github.com/gravitational/planet/pull/773), [satellite#279](https://github.com/gravitational/satellite/pull/279)).
* Update TLS cipher suites for Kubernetes components ([#2261](https://github.com/gravitational/gravity/pull/2261), [planet#780](https://github.com/gravitational/planet/pull/780)).

### 5.5.54 LTS (October 12th, 2020)

#### Improvements

* Check dependent phases when performing phase rollback to prevent cluster from entering inconsistent state ([#2185](https://github.com/gravitational/gravity/pull/2185)).

### 5.5.53 LTS (October 6th, 2020)

#### Improvements
* Gravity debug reports will include additional gravity configuration ([#2160](https://github.com/gravitational/gravity/pull/2160)).

#### Bugfixes

* Fix a security issue when using SAML authentication to an identity provider (CVE-2020-15216) ([#2193](https://github.com/gravitational/gravity/pull/2193)).
* Fix an issue with the backend not using linerizable reads ([#2179](https://github.com/gravitational/gravity/pull/2179)).
* Fix an issue with recent checks that fail during operation phases where gravity-site or etcd are unavailable ([#2164](https://github.com/gravitational/gravity/pull/2164)).
* Fix an issue with serf clusters missing nodes when partitioned for more than 24 hours ([#2174](https://github.com/gravitational/gravity/pull/2174), [planet#758](https://github.com/gravitational/planet/pull/758)).

!!! warning
    This release fixes a security vulnerability in Teleport when connecting Gravity to a SAML 2.0 identity provider. Please see
    [Teleport Announcement for CVE-2020-15216](https://github.com/gravitational/teleport/releases/tag/v4.3.7) for more information.

### 5.5.52 LTS (September 8th, 2020)

#### Improvements

* Make sure that agents are available before resuming or rolling back an upgrade ([#2044](https://github.com/gravitational/gravity/pull/2044)).
* Restart flannel before upgrade from older versions to workaround potential overlay issues ([#2043](https://github.com/gravitational/gravity/pull/2043)).
* Remove the requirement for all nodes to have the same operating system version from preflight checks ([#2048](https://github.com/gravitational/gravity/pull/2048)).
* Display active Kapacitor alerts in `gravity status` ([#2039](https://github.com/gravitational/gravity/pull/2039)).
* Update `gravity status` to display a warning if state directory ownership changes ([#2034](https://github.com/gravitational/gravity/pull/2034), [planet#727](https://github.com/gravitational/planet/pull/727), [satellite#255](https://github.com/gravitational/satellite/pull/255)).
* Validate `HTTP_PROXY` and `HTTPS_PROXY` variables set via `RuntimeEnvironment` resource ([#2049](https://github.com/gravitational/gravity/pull/2049)).
* Update the log collector rotation policy to keep up to ten 256MB log files ([#2033](https://github.com/gravitational/gravity/pull/2033), [logging#70](https://github.com/gravitational/logging-app/pull/70)).
* Drop excessive metrics from InfluxDB during an upgrade ([#2091](https://github.com/gravitational/gravity/pull/2091), [monitoring#187](https://github.com/gravitational/monitoring-app/pull/187)).

#### Bugfixes

* Fix an issue with remote_syslog process leaking CPU time ([#2084](https://github.com/gravitational/gravity/pull/2084), [logging#71](https://github.com/gravitational/logging-app/pull/71)).
* Fix an issue with etcd not being upgraded on the worker nodes ([#2086](https://github.com/gravitational/gravity/issues/2068), [planet#738](https://github.com/gravitational/planet/pull/738)).
* Fix an issue with installation sometimes failing when trying to install a cluster license ([#2053](https://github.com/gravitational/gravity/pull/2053)).
* Fix an issue with planet agent connection to serf periodically hanging after cluster networking issues ([#2079](https://github.com/gravitational/gravity/pull/2079), [planet#739](https://github.com/gravitational/planet/pull/739), [satellite#262](https://github.com/gravitational/satellite/pull/262)).

### 5.5.51 LTS (August 12th, 2020)

#### Bugfixes

* Backport upstream kubernetes fix for CVE-2020-8558 to Kubernetes 1.13 ([#1998](https://github.com/gravitational/gravity/pull/1998), [planet#650](https://github.com/gravitational/planet/pull/650)).
* Fix an issue with log forwarding configuration breaking during upgrade ([#1972](https://github.com/gravitational/gravity/pull/1972), [logging-app#67](https://github.com/gravitational/planet/pull/67)).
* Fix an issue with kubernetes scheduler priorities on cluster dns app ([#1991](https://github.com/gravitational/gravity/pull/1991)).
* Fix an issue with etcd shutdown phase during upgrades ([#1980](https://github.com/gravitational/gravity/pull/1980), [planet#718](https://github.com/gravitational/planet/pull/718)).
* Fix an issue with teleport when rotating all master servers within the cluster ([#1970](https://github.com/gravitational/gravity/pull/1970)).

!!! warning
    This release fixes a security vulnerability in Kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/g/kubernetes-announce/c/sI4KmlH3S2I) for more information.


### 5.5.50 LTS (July 31st, 2020)

#### Improvements

* Gravity debug report will now include `gravity` CLI command history ([#1858](https://github.com/gravitational/gravity/pull/1858), [#1895](https://github.com/gravitational/gravity/pull/1895)).
* Improve `gravity status` performance in case of many namespaces/services ([#1869](https://github.com/gravitational/gravity/pull/1869)).
* Improve error messages when some Teleport nodes are unavailable prior to upgrade ([#1874](https://github.com/gravitational/gravity/pull/1874)).
* Update `gravity plan resume` command to launch from a systemd service by default, `--block` flag is supported for old behavior ([#1899](https://github.com/gravitational/gravity/pull/1899)).
* Add ability to follow operation plan using `gravity plan --tail` command ([#1899](https://github.com/gravitational/gravity/pull/1899)).
* Gravity will now report a warning if installed kernel is older than the recommended `3.10.0-1127` version ([#1919](https://github.com/gravitational/gravity/pull/1919), [planet#708](https://github.com/gravitational/planet/pull/708), [satellite#249](https://github.com/gravitational/satellite/pull/249)).
* Gravity will now check tiller server health prior to upgrade ([#1916](https://github.com/gravitational/gravity/pull/1916)).
* Add ability to update Teleport auth servers using `gravity system teleport set-auth-servers` command ([#1944](https://github.com/gravitational/gravity/pull/1944)).
* Upgrade agents will now restart automatically if nodes reboot during upgrade ([#1951](https://github.com/gravitational/gravity/pull/1951)).
* Add ability to display deployed agent status using `gravity agent status` command ([#1951](https://github.com/gravitational/gravity/pull/1951)).

#### Bugfixes

* Fix an issue with InfluxDB continuous queries resulting in max series limit exhaustion ([#1838](https://github.com/gravitational/gravity/pull/1838)).
* Fix an issue with kube-dns resources potentially remaining when upgrading from 5.2 ([#1865](https://github.com/gravitational/gravity/pull/1865)).
* Fix an issue with creating custom rollups ([#1896](https://github.com/gravitational/gravity/pull/1896)).

### 5.5.49 LTS (June 25th, 2020)

#### Improvements

* Set priority classes on critical system pods ([#1692](https://github.com/gravitational/gravity/pull/1692), [planet#681](https://github.com/gravitational/planet/pull/681), [logging#65](https://github.com/gravitational/logging-app/pull/65), [monitoring#175](https://github.com/gravitational/monitoring-app/pull/175)).
* Update `gravity status` to display unhealthy critical system pods ([#1702](https://github.com/gravitational/gravity/pull/1702), [planet#682](https://github.com/gravitational/planet/pull/682), [satellite#233](https://github.com/gravitational/satellite/pull/233)).
* Display a more detailed reason for degraded status in `gravity status` ([#1707](https://github.com/gravitational/gravity/pull/1707)).
* Add `--since` flag to `gravity report` command to allow filtering collected logs by time ([#1719](https://github.com/gravitational/gravity/pull/1719)).
* Add more system information collectors to `gravity report` ([#1719](https://github.com/gravitational/gravity/pull/1719)).
* Upgrade Grafana to `v6.7.4` ([#1730](https://github.com/gravitational/gravity/pull/1730), [monitoring#176](https://github.com/gravitational/monitoring-app/pull/176)).
* Add pre-upgrade check that makes sure previous upgrade operation was fully completed or rolled back ([#1731](https://github.com/gravitational/gravity/pull/1731)).
* Increase timeout for tolerating temporary etcd issues when enabling leader elections during install and join operations ([#1797](https://github.com/gravitational/gravity/pull/1797)).

#### Bugfixes

* Fix an issue with `kube-apiserver` sometimes entering failed state during failover ([#1727](https://github.com/gravitational/gravity/pull/1727), [planet#686](https://github.com/gravitational/planet/pull/686)).
* Fix an issue with monitoring tab displaying "Dashboard not found" error ([#1730](https://github.com/gravitational/gravity/pull/1730), [monitoring#176](https://github.com/gravitational/monitoring-app/pull/176)).
* Fix an issue with upgrade failing to adjust volume permissions if only uid is specified ([#1789](https://github.com/gravitational/gravity/pull/1789)).
* Make sure upgrade is resumed on the correct node to avoid issues with plan inconsistency ([#1790](https://github.com/gravitational/gravity/pull/1790)).
* Fix an issue with disk space check executing for volumes that don't have disk requirements specified ([#1795](https://github.com/gravitational/gravity/pull/1795)).

### 5.5.48 LTS (June 15th, 2020)

#### Improvements

* Disk space check will now trigger a warning on soft limit (80%) and degrade the node on hard limit (90%) ([#1664](https://github.com/gravitational/gravity/pull/1664), [planet#672](https://github.com/gravitational/planet/pull/672), [satellite#225](https://github.com/gravitational/satellite/pull/225)).
* Add ability to update Docker device using `--docker-device` flag when upgrading from older Gravity version ([#1680](https://github.com/gravitational/gravity/pull/1680)).
* Add backoff after new cluster image upload to make sure cluster is healthy ([#1687](https://github.com/gravitational/gravity/pull/1687)).
* Move Grafana dashboards from config maps into container to avoid issues with large requests ([#1685](https://github.com/gravitational/gravity/pull/1685), [monitoring-app#173](https://github.com/gravitational/monitoring-app/pull/173)).

#### Bugfixes

* Update time drift, network health and ping checkers to not report failures during cluster modifications ([#1660](https://github.com/gravitational/gravity/pull/1660), [planet#652](https://github.com/gravitational/planet/pull/652), [satellite#214](https://github.com/gravitational/satellite/pull/214)).
* Fix an issue with calculating port differences when upgrading to a new version ([#1656](https://github.com/gravitational/gravity/pull/1656)).
* Fix an issue with not being able to determine Teleport node configuration package when upgrading from older versions ([#1677](https://github.com/gravitational/gravity/pull/1677)).
* Fix an issue with old Teleport service not being uninstalled properly when upgrading from 5.0 to 5.5 ([#1683](https://github.com/gravitational/gravity/pull/1683)).

### 5.5.47 LTS (May 29th, 2020)

#### Improvements

* Upgrade etcd to `v3.3.22` ([#1619](https://github.com/gravitational/gravity/pull/1619), [planet#650](https://github.com/gravitational/planet/pull/650)).
* Upgrade CNI plugins to `v0.8.6` (CVE-2020-10749) ([#1619](https://github.com/gravitational/gravity/pull/1619), [planet#650](https://github.com/gravitational/planet/pull/650)).

#### Bugfixes

* Fix a number of issues related to handling of operations without plan in `gravity plan` ([#1597](https://github.com/gravitational/gravity/pull/1597)).
* Fix an issue with RPC agent credentials not being rotated during upgrade ([#1629](https://github.com/gravitational/gravity/pull/1629)).

!!! warning
    This release fixes a security vulnerability in CNI. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-announce/wGuDMGdnW9M) for more information.

### 5.5.46 LTS (May 21st, 2020)

#### Improvements

* Add `gravity rollback` command to support automatic plan rollback of failed upgrade operations ([#1558](https://github.com/gravitational/gravity/pull/1558)).
* Add profiling endpoint to planet-agent ([#1579](https://github.com/gravitational/gravity/pull/1579), [planet#569](https://github.com/gravitational/planet/pull/569), [satellite#165](https://github.com/gravitational/satellite/pull/165)).
* Bump Grafana to `5.4.5` and Telegraf to `1.11.5` ([#1593](https://github.com/gravitational/gravity/pull/1593), [monitoring-app#168](https://github.com/gravitational/monitoring-app/pull/168)).
* Update monitoring containers to run as non-root and remove unnecessary binaries ([#1593](https://github.com/gravitational/gravity/pull/1593), [monitoring-app#168](https://github.com/gravitational/monitoring-app/pull/168)).

#### Bugfixes

* Re-enable nethealth checker as a warning and fix a number of issues related to removing stale data for deleted nodes ([#1580](https://github.com/gravitational/gravity/pull/1580), [planet#637](https://github.com/gravitational/planet/pull/637), [monitoring-app#166](https://github.com/gravitational/monitoring-app/pull/166)).
* Increase image pull deadline on kubelet to 15m ([#1584](https://github.com/gravitational/gravity/pull/1584), [planet#640](https://github.com/gravitational/planet/pull/640)).

### 5.5.45 LTS (May 18th, 2020)

#### Improvements

* Include `gravity status history` in the debug report ([#1570](https://github.com/gravitational/gravity/pull/1570)).

#### Bugfixes

* Strip gravity-site image from unnecessary binaries ([#1574](https://github.com/gravitational/gravity/pull/1574)).
* Fix a "role not found" issue when connecting to a leaf cluster via Hub ([#1577](https://github.com/gravitational/gravity/pull/1577)).

### 5.5.44 LTS (May 13th, 2020)

#### Improvements

* Add Teleport nodes status to `gravity status` ([#1477](https://github.com/gravitational/gravity/pull/1477)).
* Add ability to display warning probes in `gravity status` ([#1498](https://github.com/gravitational/gravity/pull/1498)).

#### Bugfixes

* Make sure Teleport node has connected successfully when joining a new node ([#1477](https://github.com/gravitational/gravity/pull/1477)).
* Fix a number of issues that may have led to crashes when trying to resume operations ([#1479](https://github.com/gravitational/gravity/pull/1479), [#1526](https://github.com/gravitational/gravity/pull/1526)).
* Fix an issue with inability to set a `role_map` property when configuring a trusted cluster ([#1556](https://github.com/gravitational/gravity/pull/1556)).
* Update default trusted cluster role mapping to only map remote admin role to local admin role ([#1556](https://github.com/gravitational/gravity/pull/1556)).

!!! warning
    This release addresses an issue with an insecure default that would map any remote role to the local admin role when connecting
    a cluster to a Hub using a trusted cluster without an explicitly configured role mapping. See [Trusted Clusters](config.md#trusted-clusters-enterprise)
    documentation for role mapping configuration details.

### 5.5.43 LTS (May 1st, 2020)

#### Bugfixes

* Restore `/usr/local/bin`, `/bin/helm` and `/bin/kubectl` symlinks in planet container ([#1483](https://github.com/gravitational/gravity/pull/1483)).

### 5.5.42 LTS (April 28th, 2020)

#### Bugfixes

* Prevent nethealth checker from affecting the cluster status temporarily to avoid possible issues with cluster becoming degraded after removing a node ([#1464](https://github.com/gravitational/gravity/pull/1464)).
* Upgrade etcd to 3.3.20 in order to fix a problem with missed etcd events ([#1464](https://github.com/gravitational/gravity/pull/1464)) ([#1436](https://github.com/gravitational/gravity/issues/1436)).

### 5.5.41 LTS (April 24th, 2020)

#### Improvements

* Add leader change event to gravity timeline ([#1355](https://github.com/gravitational/gravity/pull/1355)).
* Update kube-apiserver and kubelet to use Mozilla modern compatibility ciphers ([#1439](https://github.com/gravitational/gravity/pull/1439)).

#### Bugfixes

* Disallow running certain commands inside planet container which could lead to unexpected results ([#1353](https://github.com/gravitational/gravity/pull/1353)).
* Fix an issue with wormhole CNI plugin installation ([#1372](https://github.com/gravitational/gravity/pull/1372)).
* Fix an issue with `gravity app install` command returning an error ([#1408](https://github.com/gravitational/gravity/pull/1408)).
* Fix an issue with monitoring app upgrade silently failing sometimes ([#1428](https://github.com/gravitational/gravity/pull/1428)).
* Fix an issue with InfluxDB consuming a lot of CPU and memory when CronJobs are used ([#1428](https://github.com/gravitational/gravity/pull/1428)).
* Fix an issue with cluster controller attempting to connect to the agent after operation completion ([#1430](https://github.com/gravitational/gravity/pull/1430)).
* Fix an issue with Teleport nodes failing to join after expand operation ([#1443](https://github.com/gravitational/gravity/pull/1443)).

### 5.5.40 LTS (April 3rd, 2020)

!!! warning
    This release has a known issue that can lead to new Teleport nodes failing to join the cluster. Please use a more current release.

#### Improvements

* Add overlay network checker to the in-cluster problem detector ([#1293](https://github.com/gravitational/gravity/pull/1293)).
* Add checks for unsupported upgrade paths ([#1232](https://github.com/gravitational/gravity/pull/1232)).
* Add cgroup cleaner to planet to prevent leaking cgroups ([planet#578](https://github.com/gravitational/planet/pull/578)).
* Add `--pull` flag to `tele build` to allow always pulling latest versions of images ([#1308](https://github.com/gravitational/gravity/pull/1308)).
* Upgrade serf to `v0.8.5` ([#1314](https://github.com/gravitational/gravity/pull/1314)).

#### Bugfixes

* Fix an issue with copying lengthy commands in web terminal ([#1258](https://github.com/gravitational/gravity/pull/1258)).
* Bring back flat container log structure in debug report tarballs ([#1278](https://github.com/gravitational/gravity/pull/1278)).
* Fix an issue with displaying server version in `gravity status` ([#1304](https://github.com/gravitational/gravity/pull/1304)).
* Fix a race condition that could lead to planet rootfs being reset during upgrade ([#1301](https://github.com/gravitational/gravity/pull/1301)).
* Fix a number of issues that could lead to expand operation being stuck ([#1298](https://github.com/gravitational/gravity/pull/1298)).

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

### 5.5.21 LTS (September 26th, 2019)

!!! note "Direct upgrades"
    You can now upgrade existing 5.0.x clusters directly to 5.5.x.
    See [Direct Upgrades From Older LTS Versions](cluster.md#multi-hop-upgrades) for details.

#### Improvements

* Add support for direct upgrades of clusters based on Gravity 5.0.x ([#637](https://github.com/gravitational/gravity/pull/637)).
* Update github.com/gravitational/monitoring-app to 5.5.3 ([#644](https://github.com/gravitational/gravity/pull/644)).
  * Disable NodePort access to Kapacitor service.
  * Randomly generate passwords for superuser, telegraf, heapster and grafana users in InfluxDB database during installation and upgrades.

#### Bugfixes

* Fix an issue with `gravity report` not capturing planet journal logs ([#684](https://github.com/gravitational/gravity/pull/684)).
* Fix a package ordering issue in `tele build` ([#712](https://github.com/gravitational/gravity/pull/712)).
* Fix an issue with the time drift checker ([#710](https://github.com/gravitational/gravity/pull/710)).

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

### 5.5.15 LTS (August 6th, 2019)

#### Bugfixes

* Update etcd gateway configuration as masters are removed or added to the cluster.
* Upgrade Kubernetes to `v1.13.9` (CVE-2019-11247, CVE-2019-11249).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/Vf31dXp0EJc) for more information.

### 5.5.14 LTS (July 24th, 2019)

#### Bugfixes

* Fix an issue with managing monitoring resources via `gravity resource` command.

### 5.5.13 LTS (July 18th, 2019)

#### Bugfixes

* Workaround for installation failures when populating the docker registry.
* Fix an issue with applications that contain a resources subfolder failing to install.

#### Improvements

* Installations that previously used a dedicated devicemapper volume will now be reformatted and reused after upgrade.

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

### 5.5.11

#### Improvements

* Upgrade Kubernetes to `v1.13.6` (CVE-2019-1002101)

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://groups.google.com/forum/#!topic/kubernetes-security-discuss/zqklrWzeA2c) for more information.

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

### 5.5.7 LTS

#### Bugfixes

* Fix an issue with completed status hook pods not being cleaned up.

### 5.5.6 LTS

#### Bugfixes

* Fix an issue with adjusting user volume permissions during upgrade.

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

### 5.5.0 LTS

#### Improvements

* Multiple UX tweaks for `gravity app list` command.
* Better validation for the cloud configuration resource.

#### Bugfixes

* Fix an issue with `kubectl` not working on host when using custom state directory.
* Fix an issue with `gravity status` always displaying degraded status on regular nodes.
* Fix an issue with shrink operation sometimes spawning on the leaving node.

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

### 5.5.0-alpha.3

#### Improvements

* Add support for Wireguard encrypted overlay network. See [Wireguard Encrypted Networking](cluster.md#wireguard-encrypted-networking) for details.
* Reduce writes to AWS SSM store when using AWS integrations.
* Update tiller to 2.11.0
* Add initial support for application catalog. See [Application Catalog](catalog.md) for details.
* Update embedded teleport to 3.0.1

## 5.4 Releases

### 5.4.10

#### Improvements

* Upgrade Kubernetes to `v1.13.5` (CVE-2019-1002101).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/announce-security-release-of-kubernetes-kubectl-potential-directory-traversal-releases-1-11-9-1-12-7-1-13-5-and-1-14-0-cve-2019-1002101/5712) for more information.

### 5.4.9

#### Improvements

* Improve shrink operation behavior when using Auto-Scaling Groups on AWS.

### 5.4.7

#### Bugfixes

* Update to Kubernetes 1.13.4 (CVE-2019-1002100).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/kubernetes-security-announcement-v1-11-8-1-12-6-1-13-4-released-to-address-medium-severity-cve-2019-1002100/5147) for more information.

### 5.4.6

#### Bugfixes

* Update to Docker 18.06.2 (cve-2019-5736).

!!! warning
    This release fixes a security vulnerability in runc. Please see
    [Kubnernetes Blog](https://kubernetes.io/blog/2019/02/11/runc-and-cve-2019-5736/) for more information.

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

### 5.4.2

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

## 5.3 Releases

### 5.3.9

#### Improvements

* Use `overlay2` as default storage driver.
* Enable aggregation layer on the Kubernetes API server.

#### Bugfixes

* Fix an issue with manually completing rolled back upgrade plan.

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

### 5.3.5

#### Bugfixes

* Update Kubernetes to version 1.12.3.

!!! warning
    Kubernetes 1.12.3 includes fixes for CVE-2018-1002105. Please see
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

### 5.3.2

#### Bugfixes

* Fix an issue with cluster expansion when applying taints via app.yaml.
* Fix an issue with labeling of packages, which could prevent upgrades from completing.

#### Improvements

* Improved error message when RPC agent fails to connect.

### 5.3.1

#### Bugfixes

* Strip original registry when tagging images to local registry when using Helm charts.

### 5.3.0

#### Improvements

* Upgrade to Kubernetes `1.12.1`.
* Replace `kube-dns` with CoreDNS.
* Remove dependency on system user/group being present in local `/etc/passwd` and `/etc/group` databases.

## 5.2 Releases

### 5.2.18 LTS (October 23rd, 2020)

#### Bugfixes

* Fix an issue where etcd-backups were using too short of a timer ([#2263](https://github.com/gravitational/gravity/pull/2263), [planet#770](https://github.com/gravitational/planet/pull/770), [etcd-backup#5](https://github.com/gravitational/satellite/pull/5)).

### 5.2.17 LTS (June 11th, 2020)

#### Improvements

* Increase disk check high watermark to 90% ([#1679](https://github.com/gravitational/gravity/pull/1679), [planet#675](https://github.com/gravitational/planet/pull/675)).

### 5.2.16 LTS (October 11th, 2019)

#### Improvements

* Improves `gravity report` to capture additional network configuration ([#773](https://github.com/gravitational/gravity/pull/773)).
* Increase timeout on healthz check and allow optional configuration by environment variable ([#737](https://github.com/gravitational/gravity/pull/737)).

#### Bugfixes

* Skip missing mount points when checking filesystem usage ([#786](https://github.com/gravitational/gravity/pull/786)).

### 5.2.15 LTS (September 19th, 2019)

#### Improvements

* Update github.com/gravitational/monitoring-app to 5.2.5 ([#642](https://github.com/gravitational/gravity/pull/642)).
* Add support for intermediate upgrades ([#711](https://github.com/gravitational/gravity/pull/711), [#709](https://github.com/gravitational/gravity/pull/709), [#612](https://github.com/gravitational/gravity/pull/612)).

#### Bugfixes

* Improve OS metadata parsing in agents ([#721](https://github.com/gravitational/gravity/pull/721)).

### 5.2.14 LTS (July 30th, 2019)

#### Improvements

* Remove dependency on system user/group being present in local `/etc/passwd` and `/etc/group` databases.
* Generate credentials for InfluxDB, Telegraf and Grafana during installation and update.

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

### 5.2.12 LTS

#### Improvements

* Upgrade Kubernetes to `v1.11.9` (CVE-2019-1002101).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/announce-security-release-of-kubernetes-kubectl-potential-directory-traversal-releases-1-11-9-1-12-7-1-13-5-and-1-14-0-cve-2019-1002101/5712) for more information.

### 5.2.11 LTS

#### Bugfixes

* Fix an issue with manually completing rolled back upgrade plan.

### 5.2.10 LTS

#### Bugfixes

* Update to Kubernetes 1.13.4 (CVE-2019-1002100).

!!! warning
    This release fixes a security vulnerability in kubernetes. Please see
    [Kubernetes Announcement](https://discuss.kubernetes.io/t/kubernetes-security-announcement-v1-11-8-1-12-6-1-13-4-released-to-address-medium-severity-cve-2019-1002100/5147) for more information.

### 5.2.9 LTS

#### Bugfixes

* Fix the issue with "gravity gc" failing to collect packages on regular nodes.

### 5.2.8 LTS

#### Bugfixes

* Update docker-runc to avoid security vulnerability (cve-2019-5736).

!!! warning
    This release fixes a security vulnerability in runc. Please see
    [Kubnernetes Blog](https://kubernetes.io/blog/2019/02/11/runc-and-cve-2019-5736/) for more information.

### 5.2.7 LTS

#### Bugfixes

* Update teleport binaries to match embedded version.
* Update gravity-site healthz endpoint to more reliably indicate failures.

### 5.2.6 LTS

#### Bugfixes

* Fix an issue with cluster expansion when applying taints via app.yaml.

### 5.2.5 LTS

#### Bugfixes

* Revendor teleport to 2.4.10.

!!! warning
    Teleport 2.4.10 includes fixes for a security vulnerability. Please see
    [Teleport Announcements](https://gravitational.zendesk.com/hc/en-us/articles/360015185614-Teleport-3-1-2-3-0-3-2-7-7-2-6-10) for more information.

### 5.2.4 LTS

#### Bugfixes

* Update Kubernetes to version 1.11.5.

!!! warning
    Kubernetes 1.11.5 includes fixes for CVE-2018-1002105. Please see
    [Issue 71411](https://github.com/kubernetes/kubernetes/issues/71411) for more information.

### 5.2.3 LTS

#### Improvements

* Add support for recursive volume mounts. See [Image Manifest](pack.md#image-manifest) for details.
* Disable `DenyEscalatingExec` admission controller to meet conformance.

### 5.2.2 LTS

#### Bugfixes

* Strip original registry when tagging images to local registry when using Helm charts.

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

### 5.2.0-rc.1

#### Improvements

* Introduce gravity terraform provider.
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

### 5.2.0-alpha.1

#### Improvements

* Add `--dns-zone` flag to `gravity install` command to allow overriding upstreams
for specific DNS zones within the cluster. See flag description in the
[Installation](installation.md#cli-installation) section for details.

## 5.1 Releases

### 5.1.3

#### Bugfixes

* Revendor teleport to 2.4.10.

!!! warning
    Teleport 2.4.10 includes fixes for a security vulnerability. Please see
    [Teleport Announcements](https://gravitational.zendesk.com/hc/en-us/articles/360015185614-Teleport-3-1-2-3-0-3-2-7-7-2-6-10) for more information.

### 5.1.2

#### Bugfixes

* Update Kubernetes to version 1.9.12-gravitational.

!!! warning
    Gravitational has backported the fix for CVE-2018-1002105 to kubernetes version 1.9.12-gravitational. Please see
    [Issue 71411](https://github.com/kubernetes/kubernetes/issues/71411) for more information.

### 5.1.1

#### Improvements

* Speed up image vendoring during tele build.
* Add cleanup phase to the upgrade operation.
* Make new application upload more efficient.

#### Bugfixes

* Fix tele build failure when encountering unrecognized resources.

### 5.1.0

#### Improvements

* Update `kube-dns` application to version 1.14.10.
* Preflight checks are executed on expand.

#### Bugfixes

* Fix OS distribution detection for RedHat when lsb_release is installed.
* Fix an issue when configured cloud provider was not propagated to agents.

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

### 5.1.0-alpha.1

#### Improvements

* Add support for GCE cloud provider. See [Installing on Google Compute Engine](installation.md#google-compute-engine)
for details.

#### Bugfixes

* Fix an issue with `--force` flag not propagating correctly when resuming
install/upgrade.
* Add `NoExecute` taint toleration to hooks.

## 5.0 Releases

### 5.0.36 LTS (June 8th, 2020)

#### Bugfixes

* Fix an issue where `--docker-device` wasn't able to follow symlinks to block devices ([gravity.e#4301](https://github.com/gravitational/gravity.e/pull/4301)).

### 5.0.35 (September 2nd, 2019)

#### Bugfixes

* Upgrade golang to `v1.11.13` (CVE-2019-9512, CVE-2019-9514)
* Upgrade Kubernetes to `v1.9.13-gravitational` (CVE-2019-9512, CVE-2019-9514).

!!! warning
    Gravitational has backported the fix for CVE-2019-9512 and CVE-2019-9514 to kubernetes version 1.9.13-gravitational.
    This release fixes a security vulnerability in golang used by gravity and kubernetes. Please see
    [Netflix Announcement](https://github.com/Netflix/security-bulletins/blob/master/advisories/third-party/2019-002.md) for more information.

### 5.0.33 LTS

#### Bugfixes

* Fix a regression with `gravity upgrade --complete`

### 5.0.32 LTS

#### Bugfixes

* Fix an issue with upgrades for applications that were packaged with multiple versions of planet.

### 5.0.30 LTS

#### Improvements

* Improve resiliency of the election install phase.

### 5.0.29

#### Bugfixes

* Update docker-runc to avoid security vulnerability (cve-2019-5736).
* Update xterm.js to avoid security vulnerability (CVE-2019-0542).
* Restrict Teleport cipher suites.

!!! warning
    This release fixes a security vulnerability in runc. Please see
    [Kubnernetes Blog](https://kubernetes.io/blog/2019/02/11/runc-and-cve-2019-5736/) for more information.

### 5.0.28

#### Bugfixes

* Revendor teleport to 2.4.10.

!!! warning
    Teleport 2.4.10 includes fixes for a security vulnerability. Please see
    [Teleport Announcements](https://gravitational.zendesk.com/hc/en-us/articles/360015185614-Teleport-3-1-2-3-0-3-2-7-7-2-6-10) for more information.

### 5.0.27 LTS

#### Bugfixes

* Fix an issue with upgrade failure on clusters with non-master nodes.

### 5.0.26 LTS

#### Bugfixes

* Fix an issue with upgrade agents failing to start in some cases.

### 5.0.25

#### Bugfixes

* Update Kubernetes to version 1.9.12-gravitational.

!!! warning
    Gravitational has backported the fix for CVE-2018-1002105 to kubernetes version 1.9.12-gravitational. Please see
    [Issue 71411](https://github.com/kubernetes/kubernetes/issues/71411) for more information.

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

### 5.0.4 LTS

#### Bugfixes

* Fix an issue with `--force` flag not propagating correctly when resuming
install/upgrade.

### 5.0.3 LTS

#### Bugfixes

* Exclude Docker device test from upgrade preflight checks.
* Fix an issue with `kubectl` not working from host.

### 5.0.2 LTS

#### Bugfixes

* Monitoring: fix an RBAC permission issue with collecting metrics from heapster.

### 5.0.1 LTS

#### Bugfixes

* Fix an issue with using custom `--state-dir` when installing on more than a single node.

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

!!! tip "Cluster certificates expiration"
    If you have a Gravity cluster of version before `5.0.0-alpha.12` that
    hasn't been upgraded in about a year, its certificates may be expiring soon.
    If you are unable to upgrade, or your cluster certificates have already
    expired, please refer to the [Gravity Manual Certificates Renewal](https://gravitational.zendesk.com/hc/en-us/articles/360000755967-Telekube-Manual-Certificates-Renewal)
    article in our Help Center.

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
  [section](pack.md#image-manifest)

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

!!! warning
    Upgrading clusters to Gravity 4.23.0 works via the command line interface (CLI) only.
    To upgrade a cluster with an application packaged with the Gravity 4.23+
    follow the procedure below.

#### Notes

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

#### Bugfixes

* Use dns to access gravity-site when running non-kube-system hooks

### 3.43.0

#### Bugfixes

* Enable network.target [dependency for planet](https://gravitational.zendesk.com/hc/en-us/articles/115008045447)

### 3.42.0

#### Features

* Add 5 and 6 node flavors to gravity bundle
* Don't exit interactive installer after failure. This will allow to collect crashreports via UI.

### 3.41.0

#### Features

* Add lvm system directory to agent heartbeat to improve install experience.

### 3.40.0

#### Bugfixes

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

#### Bugfixes

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

#### Features

* Add opscenter section to documentation

### 1.25.0

#### Bugfixes

* Fix trusted authorities ACL method

### 1.24.0

#### Features

* Check tele/runtime version compatibility on build

### 1.23.0

#### Bugfixes

* Remove ls/sh commands and their docs

### 1.22.0

#### Bugfixes

* Delete tunnel after the install
* Fix a channel test that would intermittently block
