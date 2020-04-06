/*
Copyright 2020 Gravitational, Inc.

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

package ops

import (
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// NewOperation creates a new operation resource from storage operation.
func NewOperation(op storage.SiteOperation) storage.Operation {
	operation := &storage.OperationV2{
		Kind:    storage.KindOperation,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      op.ID,
			Namespace: defaults.Namespace,
		},
		Spec: storage.OperationSpecV2{
			Type:    op.Type,
			Created: op.Created,
			Updated: op.Updated,
			State:   op.State,
		},
	}
	switch op.Type {
	case OperationInstall:
		operation.Spec.Install = &storage.OperationInstall{
			Nodes: newNodes(op.Servers),
		}
	case OperationExpand:
		operation.Spec.Expand = &storage.OperationExpand{
			Node: newNode(op.Servers[0]),
		}
	case OperationShrink:
		operation.Spec.Shrink = &storage.OperationShrink{
			Node: newNode(op.Shrink.Servers[0]),
		}
	case OperationUpdate:
		operation.Spec.Upgrade = &storage.OperationUpgrade{
			Package: op.Update.UpdatePackage,
		}
	case OperationUpdateRuntimeEnviron:
		operation.Spec.UpdateEnviron = &storage.OperationUpdateEnviron{
			Env: op.UpdateEnviron.Env,
		}
	case OperationUpdateConfig:
		operation.Spec.UpdateConfig = &storage.OperationUpdateConfig{
			Config: op.UpdateConfig.Config,
		}
	case OperationReconfigure:
		operation.Spec.Reconfigure = &storage.OperationReconfigure{
			IP: op.Reconfigure.AdvertiseAddr,
		}
	}
	return operation
}

func newNodes(servers []storage.Server) (nodes []storage.OperationNode) {
	for _, server := range servers {
		nodes = append(nodes, newNode(server))
	}
	return nodes
}

func newNode(server storage.Server) storage.OperationNode {
	return storage.OperationNode{
		IP:       server.AdvertiseIP,
		Hostname: server.Hostname,
		Role:     server.Role,
	}
}
