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

package expand

import (
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
)

/*

PHASES:

/prechecks

/configure -> gravity-site configures packages

/bootstrap -> setup directories/volumes on the new node, devicemapper, log into site

/pull -> pull configured packages on the new node

/pre -> run preExpand hook

/etcd -> add etcd member

/system -> install teleport/planet units

/wait
  /planet -> wait for planet to come up and check etcd cluster health
  /k8s -> wait for new node to register with k8s

/post -> run postExpand hook

*/

func NewOperationPlan(operation ops.SiteOperation) (*storage.OperationPlan, error) {
}
