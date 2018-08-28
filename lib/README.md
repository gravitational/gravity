### lib

Lib implements a set of disjointed services that can work in one
process or several processes on remote machine based on deployment type.

Every service is organized int the same way:

```
<name>/            // service interfaces and structs, access control wrappers
   <nameservice>   // actual implementation of the service
   <namehandler>   // HTTP server web handlers
   <nameclient>    // HTTP thin client
   <suite>         // test suite that is used to test client and service using interfaces
```

for example,

```
ops
  opsservice
  opsclient
  opshandler
  suite
```


Here are key services we have:

* ops - operations service is responsible for creating, deleting and updating sites.
* pack - responsible for application and low level system packages distribution and updates
* app - responsible for K8s specific application management once it is in K8s
* users - manages users and permissions, roles and access control

Other packages that are not services

* systemservice - utility methods for intergration with systemd
* httplib - gravity specific HTTP wrappers
* schema - defines app.yaml schema and manifest
* storage - abstraction for storage backends, implements SQLite for development. All business logic is in services, storage is dumb
* virsh - tools for working with virsh
* portal - is a thin layer that starts (ops, pack, app, users) and teleport services, has no actual logic

  
