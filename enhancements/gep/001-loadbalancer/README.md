# Gravity Enhancement Proposal

## GEP-001: Change DNS balancer to a software loadbalancer (HAProxy)

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
    - [User Stories (Optional)](#user-stories-optional)
        - [Story 1](#story-1)
        - [Story 2](#story-2)
    - [Notes/Constraints/Caveats (Optional)](#notesconstraintscaveats-optional)
    - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
    - [Test Plan](#test-plan)
- [Production Readiness Review Questionnaire](#production-readiness-review-questionnaire)
    - [Feature Enablement and Rollback](#feature-enablement-and-rollback)
- [Implementation History](#implementation-history)

<!-- /toc -->

## Summary
The main goal of this proposal is to replace dns balancing with a software loadbalancer.
Thus, all requests to the kube-apiserver from Kubernetes components (kubelet, scheduler, control-manager, kube-proxy)
will go through this loadbalancer. Also, requests to the docker registry must also go through the loadbalancer.

## Motivation
At the moment, Gravity is using CoreDNS to determine which master to send requests to,
but DNS is not a good tool for balancing traffic.
This proposal aims to provide the following improvements:
  1. Evenly distribute the load between all apiserver replicas
  1. Improve the stability and scalability of the cluster
  1. Nodes will react faster to master failures
  1. Cluster upgrades will be faster and more efficient

### Goals
When creating a cluster, the user can choose the load balancer to be either external or internal.
External loadbalancer is not controlled or configured by Gravity, it is the user's responsibility.
The internal loadbalancer, on the contrary, is controlled and configured by Gravity, the planet-agent is responsible for this.
When using an internal loadbalancer, a loadbalancer (HAProxy) will be installed for each node in the cluster including the master nodes.
When adding/removing master nodes, the agent on each node should automatically reflect this in the local loadbalancer configuration.
All Kubernetes components (kubelet, scheduler, control-manager, kube-proxy) will use the loadbalancer to access the apiserver.

### Non-Goals

## Proposal
Change DNS balancer to a software loadbalancer (HAProxy).
CoreDNS will resolve `leader.telekube.local` to either the external address (domain name or IP) of the loadbalancer (subject to which configuration?) or to `127.0.0.1` when using the internal loadbalancer.
Docker image references should continue pointing to internal registry at `leader.telekube.local:5000`.
To achieve this, docker registry will use port 5001 with port 5000 left to the loadbalancer.

### User Stories
#### Story 1
As a cluster administrator, I can configure the loadbalancer when creating a cluster. If I set the type of loadbalancer to external, I can provide the external address in configuration.
#### Story 2
As a cluster administrator, I can change the loadbalancer settings on a running cluster.

### Risks and Mitigations
As the `leader.telekube.local` domain will not point to a master node, the applications using this address should be updated accordingly.

## Design Details
An additional section will be added to the cluster manifest for setting up the loadbalancer:
```yaml
kind: Cluster
apiVersion: cluster.gravitational.io/v2
loadbalancer:
  # default value is internal
  type: "external|internal"
  # gravity uses this field when type is external
  externalAddress: "IP address | DNS address"
```
The following section will be added to the cluster configuration:
```yaml
kind: ClusterConfiguration
version: v1
spec:
  loadbalancer:
    # default value is internal
    type: "external|internal"
    # gravity uses this field when type is external
    externalAddress: "IP address | DNS address"
```
The docker image references should not be changed, and they should start with `leader.telekube.local:5000/`. 
So docker registries on master nodes by default will use port 5001 with 5000 reserved for the loadbalancer. 
The domain `leader.telekube.local` will either resolve to 127.0.0.1 
for the internal loadbalancer or the address of the external loadbalancer.

Kubeconfig files will either use 127.0.0.1 for the internal loadbalancer or the address of the external loadbalancer to point to the api server.

For an external loadbalancer:
The user is responsible for configuring the loadbalancer to open the following ports
  1. for kube-apiserver: 9443 -> (master ip addresses): 6443
  1. for docker-registry: 5000 -> (master ip addresses): 5001

For an internal load balancer:
The software load balancer is haproxy, it will run on each node in the cluster.
Port 9443 will be used for load-balancing kube-apiserver and port 5000 will be used for load-balancing the docker registry.
The planet agent on master nodes will monitor the node IP address in etcd using 
the key `/planet/cluster/${KUBE_CLUSTER_ID}/masters/${MASTER_IP}` with a TTL 4 of hours.
The planet agent on each node, including master nodes will be watching all keys in 
`/planet/cluster/${KUBE_CLUSTER_ID}/masters/` to change the haproxy configuration and reload it if necessary.
TTL is needed to automatically remove master nodes from load balancing if those nodes are removed and no longer respond.

### Test Plan
- [ ] Install the cluster with the default configuration for the load balancer
- [ ] Install the cluster with the external load balancer
- [ ] Upgrade the cluster from a previous version to a version with an internal load balancer
- [ ] Upgrade a cluster from a previous version to a version with an external load balancer
- [ ] Change the type for the working cluster: internal(default) -> external -> internal

## Production Readiness Review Questionnaire
### Feature Enablement and Rollback
This feature will be enabled by default.
If the user wants to disable it, the cluster should be rolled back to an older version of Gravity.

## Implementation History
2021-07-21 Initial GEP merged
