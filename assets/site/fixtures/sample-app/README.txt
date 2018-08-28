This is a sample "fixture" application to test "gravity install".
It features:
    - mix of yaml and json object definitions
    - arbitrary subdirectories
    - two real docker images 
    - random data (like this README file) which will be ignored

App structure:

Pods
----
1. Sample-app pod
    - includes "busy-bash" image
    - includes "sample-app" image
2. Bash pod
    - includes "busy-bash" image


Services
--------
1. Etcd service
2. Sample-app service


Endpoints
---------
1. Etcd service endpoints
