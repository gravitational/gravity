---
title: Monitoring a Kubernetes Cluster with Gravity
description: How to monitor the health and performance an air-gapped or on-prem Kubernetes cluster with Gravity
---

# Cluster Monitoring

Gravity Clusters come with a fully configured and customizable monitoring/alerting system by default.
The monitoring stack consists of the following components: Prometheus, Grafana, Alertmanager and
Satellite.

These components are automatically included into a Cluster Image built with `tele build` as a
system dependency (see the [source](https://github.com/gravitational/monitoring-app) on GitHub).


**Example Monitoring Dashboard**
![Set Capacity](images/gravity-quickstart/gravity-monitoring.png)

### Prometheus


[Prometheus](https://prometheus.io/docs/introduction/overview/) is an open-source Kubernetes
native monitoring system and time-series database.

Prometheus uses the following in-cluster services to collect the metrics about the Cluster:

* [node-exporter](https://github.com/prometheus/node_exporter) collects hardware and OS metrics.
* [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) collects metrics about
various Kubernetes resources such as deployments, nodes and pods.

!!! note 
    Collected metrics are stored for 30 days.

Prometheus exposes the cluster-internal service `prometheus-k8s.monitoring.svc.cluster.local:9090`.

### Grafana

[Grafana](https://grafana.com/) is an open-source metrics analytics and visualization suite.

In Gravity Clusters Grafana uses Prometheus as a data-source. It is preconfigured with several
dashboards that provide general information about individual nodes, containers and the overall
Cluster health.

!!! tip
    When building a Cluster Image, it is possible to add your own dashboards in addition to
    the ones that ship by default. See [Grafana Integration](#grafana-integration) below for
    details.

Grafana exposes the cluster-internal service `grafana.monitoring.svc.cluster.local:3000`.

### Alertmanager

[Alertmanager](https://prometheus.io/docs/alerting/alertmanager/) is a Prometheus component that
handles alerts sent by Prometheus server and takes care of deduplicating, grouping and routing
them to the correct receiver such as email recipient.

Alertmanager exposes the cluster-internal service `alertmanager-main.monitoring.svc.cluster.local:9093`.

### Satellite

[Satellite](https://github.com/gravitational/satellite) is an open-source problem detector tool
for Kubernetes clusters developed by Gravitational.

Satellite runs on each Gravity Cluster node and executes a multitude of checks continuously
assessing the health of the Cluster and the individual nodes. Any issues detected by Satellite
will be displayed in the output of the `gravity status` command.

See [Cluster Status](cluster.md#cluster-status) for more information.

## Grafana Integration

The default Grafana configuration includes two pre-configured dashboards providing machine- and
pod-level overview of the installed Cluster. Grafana UI is integrated with Gravity Control Panel.
To view dashboards once the Cluster is up and running, navigate to the Cluster's Monitoring page.

In Gravity clusters Grafana is running in anonymous read-only mode which allows anyone logged
into Gravity to view existing dashboards but not modify them or create new ones.

## Pluggable Dashboards

Your Cluster Image can include its own Grafana dashboards using ConfigMaps.

A custom dashboard ConfigMap should be placed into the `monitoring` namespace and assigned the
special `monitoring: dashboard` label so it is recognized as a dashboard and loaded during initial
Cluster Image installation:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mydashboard
  namespace: monitoring
  labels:
    monitoring: dashboard
data:
  mydashboard: |
    { ... dashboard JSON ... }
```

Dashboard ConfigMap may contain multiple keys with dashboards and key names are not relevant. The monitoring
application source on GitHub has an [example](https://github.com/gravitational/monitoring-app/blob/5.2.1/resources/grafana.yaml#L395)
of a dashboard ConfigMap.

!!! tip
    Since the embedded Grafana runs in read-only mode, you can use a separate Grafana instance
    to create a custom dashboard and then export it as JSON.

## Alertmanager Integration

### Configuring Alerts Delivery

To configure Alertmanager to send email alerts, you need to create two Gravity resources.

The first resource is called `smtp`. It defines configuration of SMTP server to use:

```yaml
kind: smtp
version: v2
metadata:
  name: smtp
spec:
  host: smtp.host
  port: <smtp port>
  username: <username>
  password: <password>
```

Create the SMTP configuration:

```bash
$ gravity resource create smtp.yaml
```

The second resource is called `alerttarget`. It defines the alerts email recipient:

```yaml
kind: alerttarget
version: v2
metadata:
  name: email-alerts
spec:
   # email address of the alerts recipient
  email: triage@example.com
```

Create the target:

```bash
$ gravity resource create target.yaml
```

!!! note 
    Currently only a single alerts email recipient is supported.

### Configuring Alerts

Defining new alerts is done via a Gravity resource called `alert`:

```yaml
kind: alert
version: v2
metadata:
  name: cpu-alert
spec:
  # the alert name
  alert_name: CPUAlert
  # the rule group the alert belongs to
  group_name: test-group
  # the alert expression
  formula: |
    node:cluster_cpu_utilization:ratio * 100 > 80
  # the alert labels
  labels:
    severity: info
  # the alert annotations
  annotations:
    description: |
      Cluster CPU usage exceeds 80%.
```

!!! tip
    See [Alerting Rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)
    documentation for more details about Prometheus alerts.

Create the alert:

```bsh
$ gravity resource create alert.yaml
```

View existing alerts:

```bsh
$ gravity resource get alerts
```

Remove an alert:

```bsh
$ gravity resource rm alert cpu-alert
```

### Builtin Alerts

The following table shows the alerts Gravity ships with by default:

| Component   | Alert         | Description      |
| ------------- | -------------------- | -------------------- |
| CPU      | High CPU usage     | Triggers a warning, when > 75% used, with > 90% used, triggers a critical error |
| Memory | High memory usage     | Triggers a warning, when > 80% used, with > 90% used, triggers a critical error |
| Systemd      | Overall systemd health     |  Triggers an error when systemd detects a failed service |
| Systemd      | Individual systemd unit health     | Triggers an error when a systemd unit is not loaded/active |
| Filesystem | High disk space usage | Triggers a warning, when > 80% used, with > 90% used, triggers a critical error |
| Filesystem | High inode usage | Triggers a warning, when > 90% used, with > 95% used, triggers a critical error |
| System | Uptime | Triggers a warning when a node's uptime is less than 5min |
| System | Kernel parameters | Triggers an error if a parameter is not set. See [value matrix](requirements.md#kernel-module-matrix) for details. |
| Etcd | Etcd instance health | Triggers an error when an Etcd master is down longer than 5min |
| Etcd | Etcd latency check | Triggers a warning, when follower <-> leader latency exceeds 500ms, then an error when it exceeds 1s over a period of 1min |
| Docker | Docker daemon health | Triggers an error when docker daemon is down |
| Kubernetes | Kubernetes node readiness | Triggers an error when the node is not ready |
