# Cluster Monitoring

Telekube Clusters come with a fully configured and customizable monitoring/alerting system by default.
This system consists of the following components: Heapster, InfluxDB, Grafana and Kapacitor.

These components are automatically included into an Application Bundle built with `tele build` as a
system dependency (see the [source](https://github.com/gravitational/monitoring-app) on GitHub).

### Heapster

Heapster monitors Kubernetes components and reports statistics and information to InfluxDB about nodes
and pods.

### InfluxDB

InfluxDB is the main data store for current + future monitoring time series data. It provides the
service `influxdb.kube-system.svc.cluster.local`.

### Grafana

Grafana is the dashboard system that provides visualization information on all the information stored
in InfluxDB. It is exposed as the service `grafana.kube-system.svc.cluster.local`. Grafana credentials
are generated during initial installation and placed into a secret `grafana` in `kube-system` namespace.

### Kapacitor

Kapacitor is the alerting system that streams data from InfluxDB and sends alerts as configured by
the end user. It exposes the service `kapacitor.kube-system.svc.cluster.local`.

## Grafana integration

The standard Grafana configuration includes two pre-configured dashboards providing machine- and pod-level overview
of the installed cluster by default. The Grafana UI is integrated with Telekube control panel. To view dashboards once
the cluster is up and running, navigate to the Cluster's Monitoring page.

By default, Grafana is running in anonymous read-only mode which allows anyone logged into Telekube to view existing
dashboards (but not modify them or create new ones).

## Pluggable dashboards

Your applications can use their own Grafana dashboards using ConfigMaps.
A custom dashboard ConfigMap should be assigned a `monitoring` label with value `dashboard` and created in the
`kube-system` namespace so it is recognized and loaded when installing the application:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mydashboard
  namespace: kube-system
  labels:
    monitoring: dashboard
data:
  mydashboard: |
    { ... dashboard JSON ... }
```

Dashboard ConfigMap may contain multiple keys with dashboards and key names are not relevant. The monitoring
application source on GitHub has an [example](https://github.com/gravitational/monitoring-app/blob/3.0.0/resources/resources.yaml#L194-L404)
of a dashboard ConfigMap.

Since the embedded Grafana runs in read-only mode, a separate Grafana instance is required to
create a custom dashboard, which can then be exported.

## Metrics collection

All default metrics collected by heapster go into the `k8s` database in InfluxDB. All other applications that collect
metrics should submit them into the same database in order for proper retention policies to be enforced.

## Retention policies

By default InfluxDB has 3 pre-configured retention policies:

* default = 24h
* medium = 4w
* long = 52w

The `default` retention policy is supposed to store high-precision metrics (for example, all default metrics collected
by heapster with 10s interval). The `default` policy is default for `k8s` database which means that metrics that do not
specify retention policy explicitly go in there. The other two policies - `medium` and `long` are intended to store
metric rollups and should not be used directly.

Durations for each of the retention policies can be configured through the Telekube Cluster control panel.

## Rollups

Metric rollups provide access to historical data for longer time period but at lower resolution.

The monitoring system allows for the configuration of two "types" of rollups for any collected metric.

* `medium` rollup aggregates (or filters) data over a 5-minute interval and goes into `medium` retention policy
* `long` rollup aggregates (or filters) data over a 1-hour interval and goes into `long` retention policy

Rollups are pre-configured for some of the metrics collected by default. Applications that collect their own
metrics can configure their own rollups as well through ConfigMaps.

A custom rollup ConfigMap should be assigned a `monitoring` label with value `rollup` and created in the `kube-system`
namespace for it to be recognized and loaded. Below is a sample ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: myrollups
  namespace: kube-system
  labels:
    monitoring: rollup
data:
  rollups: |
    [
      {
        "retention": "medium",
        "measurement": "cpu/usage_rate",
        "name": "cpu/usage_rate/medium",
        "functions": [
          {
            "function": "max",
            "field": "value",
            "alias": "value_max"
          },
          {
            "function": "mean",
            "field": "value",
            "alias": "value_mean"
          }
        ]
      }
    ]
```

Each rollup is a JSON object with the following fields:

* `retention` - name of the retention policy (and hence the aggregation interval) for this rollup, can be medium or long
* `measurement` - name of the metric for the rollup (i.e. which metric is being "rolled up")
* `name` - name of the resulting "rolled up" metric
* `functions` - list of rollup functions to apply to metric measurement
* `function` - function name, can be mean, median, sum, max, min or percentile_XXX, where 0 <= XXX <= 100
* `field` - name of the field to apply rollup function to (e.g. "value")
* `alias` - new name for the rolled up field (e.g. "value_max")

## Kapacitor integration

Kapacitor provides alerting for default and user-defined alerts.

### Configuration

To configure Kapacitor to send email alerts, create resources of type `smtp` and `alerttarget`:

```yaml
kind: smtp
version: v2
metadata:
  name: smtp
spec:
  host: smtp.host
  port: <smtp port> # 465 by default
  username: <username>
  password: <password>
---
kind: alerttarget
version: v2
metadata:
  name: email-alerts
spec:
  email: triage@example.com # Email address of the alert recipient
```

```shell
$ gravity resource create -f smtp.yaml
```

To create new alerts, use another resource of type `alert`:


```yaml
kind: alert
version: v2
metadata:
  name: my-formula
spec:
  formula: |
    Kapacitor formula
```

And introduce it with:

```shell
$ gravity resource create -f formula.yaml
```

To view SMTP configuration or alerts:

```shell
$ gravity resource get smtps smtp
$ gravity resource get alert my-formula
```

To remove an alert:

```shell
$ gravity resource rm alert my-formula
```

### Builtin Alerts

Following table shows the alerts Telekube ships with by default:

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
| Etcd | Etcd instance health | Triggers an error when an etcd master is down longer than 5min |
| Etcd | Etcd latency check | Triggers a warning, when follower <-> leader latency exceeds 500ms, then an error when it exceeds 1s over a period of 1min |
| Docker | Docker daemon health | Triggers an error when docker daemon is down |
| InfluxDB | InfluxDB instance health | Triggers an error when InfluxDB is inaccessible |
| Kubernetes | Kubernetes node readiness | Triggers an error when the node is not ready |

Kapacitor will also trigger an email for each of the events listed above if stmp resource has been
configured (see [configuration](monitoring.md#configuration) for details).


### Custom and default alerts

Alerts (written in [TICKscript](https://docs.influxdata.com/kapacitor/v1.2/tick)) are automatically detected, loaded and
enabled. They are read from the Kubernetes ConfigMap named `kapacitor-alerts`. To create new alerts, add your alert scripts
as new key/values to that ConfigMap and reload any running Kapacitor pods.
