# Feature availability based on ACL and config flags

## Settings features
||Kind.User|Kind.Role|Kind.Cluster|Kind.License|Kind.AuthConnector|Kind.LogForwarder
|--- |--- |--- |--- |--- |--- |--- |
|Users|List||||
|Roles||List|||
|Log Forwarders||||||List
|Auth|||||List|
|License||||Create|
|Certificate|||||
|Monitoring|||||

## Cluster features
||Kind.User|Kind.Role|Kind.Cluster|Kind.License|Kind.AuthConnector|
|--- |--- |--- |--- |--- |--- |
|Kubeneters|||Connect||
|ConfigMaps|||Connect||
|Monitoring|||Connect||


## OpsCenter features
||Kind.User|Kind.Role|Kind.Cluster|Kind.License|Kind.AuthConnector|
|--- |--- |--- |--- |--- |--- |
|Table with Clusters|||List||


# Feature availability based on mode

## Settings features
|Mode|Feature|
|--- |--- |
|OpsCenter (dev)| auth, account, users, roles, license |
|OpsCenter|certificate, auth, account, users, roles, license |
|Cluster|certificate, auth, account, users, roles, log forwarders, monitoring|

# Features that can be disabled from webconfig

## Settings features
Monitoring, Log Forwarder, License

## Cluster features
ConfigMaps, Kubeneters, Monitoring, Logs