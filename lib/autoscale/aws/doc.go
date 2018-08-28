/* package aws implements autoscaling integration for AWS cloud provider

Design
------

                      +-------------------------+           +--------------------------+
                      |                         |           |                          |
                      |                         |           |                          |
                      |   Telekube Master Node  |           |    Auto Scaling Group    +---------------------------------+
                      |                         |           |                          |                                 |
                      |                         |           |                          |                                 |
                      ++-----+------------------+           +--------------------------+                                 |
                       |     |                                               +---------------+                           |
                       |     |                                               |               |                           |
                       |     | Publish Telekube ELB address                  |               |                           |
                       |     | Publish Encrypted Join Token                  |  AWS Instance |                           |
Read SQS notifications |     |                                               |               |                           |
Remove deleted nodes   |     |                                               |               |                           |
                       |     |                                               +--------+------+                           |   Push Scale Up/Scale Down Events
                       |     |                                                        |                                  |   to SQS service
                       |     |              +--------------------------+              | Discover Telekube ELB            |
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
* Autoscaler publishes Telekube load balancer service and encrypted join token
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
