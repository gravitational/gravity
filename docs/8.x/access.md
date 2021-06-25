---
title: Remotely Accessing a Kubernetes Cluster
description: How to remotely access and troubleshoot an air-gapped or on-prem Kubernetes cluster with Gravity
---

# Cluster Access

Under the hood, Gravity uses [Teleport](https://gravitational.com/teleport) to
manage Cluster Access. Teleport is an open source privileged management
solution for both SSH and Kubernetes and it comes bundled with Gravity.

Teleport manages SSH identities and access permissions to a Gravity Cluster. 
Teleport acts as a certificate authority (CA) capable of creating short-lived SSH certificates and Kubernetes certificates for remotely accessing clusters.

You can read more about how to configure access to Gravity Clusters in the 
[Cluster Configuration section about configuring access](config.md#cluster-access).

For more information, the [Teleport Architecture Document](http://gravitational.com/teleport/docs/architecture/)
covers these topics in depth.

For managing access to multiple Clusters, Gravity Enterprise comes with 
Gravity Hub, which allows users to [remotely access any Cluster that is connected 
to the Hub](hub/#remote-cluster-management).

## Logging into a Cluster

To login into Gravity cluster, use the `tsh login` command:

```bash
$ tsh --proxy=cluster.example.com login
```

The login command will open the web browser and users will have to authenticate
with a username and password or through an SSO process.

You can read more about using `tsh` in the [Teleport User Manual](https://gravitational.com/teleport/docs/user-manual/).
