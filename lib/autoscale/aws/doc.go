/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
package aws implements autoscaling integration for AWS cloud provider

Design
------

                      +-------------------------+           +--------------------------+
                      |                         |           |                          |
                      |                         |           |                          |
                      |   Gravity  Master Node  |           |    Auto Scaling Group    +---------------------------------+
                      |                         |           |                          |                                 |
                      |                         |           |                          |                                 |
                      ++-----+------------------+           +--------------------------+                                 |
                       |     |                                               +---------------+                           |
                       |     |                                               |               |                           |
                       |     | Publish Gravity  ELB address                  |               |                           |
                       |     | Publish Encrypted Join Token                  |  AWS Instance |                           |
Read SQS notifications |     |                                               |               |                           |
Remove deleted nodes   |     |                                               |               |                           |
                       |     |                                               +--------+------+                           |   Push Scale Up/Scale Down Events
                       |     |                                                        |                                  |   to SQS service
                       |     |              +--------------------------+              | Discover Gravity  ELB            |
                       |     |              |                          |              | Read Join Token                  |
                       |     +-------------->    SSM Parameter Store   <--------------+ Join the cluster                 |
                       |                    |                          |                                                 |
                       |                    +--------------------------+                                                 |
                       |                                                                                                 |
                       |                   +-------------------------------------------+                                 |
                       |                   |                                           |                                 |
                       |                   |     AWS Lifecycle Hooks/SQS notifications |                                 |
                       +------------------->                                           <---------------------------------+
                                           |                                           |
                                           +-------------------------------------------+


* Autoscaler runs on master nodes
* Autoscaler publishes the Gravity load balancer service and encrypted join token
  to the SSM (AWS systems manager) parameter store
* Instances started up as a part of auto scaling group discover the cluster
  by reading SSM parameters from the parameter store.
* Whenever ASG scales down it sends a notification to the SQS (Amazon Simple Queue Service)
queue associated with the cluster.
* Autoscaler receives the notification on the scale down and removes the node
from the cluster in forced mode (as the instance is offline by the time
notification is received)

*/
package aws
