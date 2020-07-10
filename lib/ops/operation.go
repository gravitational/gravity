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
	"fmt"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// NewOperation creates a new operation resource from storage operation.
func NewOperation(op storage.SiteOperation) (storage.Operation, error) {
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
			State:   op.State,
		},
	}
	switch op.Type {
	case OperationInstall:
		operation.Spec.Install = &storage.OperationInstall{
			Nodes: newNodes(op.Servers),
		}
	case OperationExpand:
		if len(op.Servers) != 0 {
			operation.Spec.Expand = &storage.OperationExpand{
				Node: newNode(op.Servers[0]),
			}
		}
	case OperationShrink:
		if len(op.Shrink.Servers) != 0 {
			operation.Spec.Shrink = &storage.OperationShrink{
				Node: newNode(op.Shrink.Servers[0]),
			}
		}
	case OperationUpdate:
		locator, err := loc.ParseLocator(op.Update.UpdatePackage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		operation.Spec.Upgrade = &storage.OperationUpgrade{
			Package: *locator,
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
	return operation, nil
}

// DescribeOperation returns a human friendly description of the operation.
func DescribeOperation(o storage.Operation) string {
	switch o.GetType() {
	case OperationInstall:
		return fmt.Sprintf("%v-node install",
			len(o.GetInstall().Nodes))
	case OperationExpand:
		if o.GetExpand().Node.IP != "" {
			return fmt.Sprintf("Join node %s as %v",
				o.GetExpand().Node, o.GetExpand().Node.Role)
		}
		return "Join node"
	case OperationShrink:
		return fmt.Sprintf("Remove node %s",
			o.GetShrink().Node)
	case OperationUpdate:
		return fmt.Sprintf("Upgrade to version %v",
			o.GetUpgrade().Package.Version)
	case OperationUpdateRuntimeEnviron:
		return "Runtime environment update"
	case OperationUpdateConfig:
		return "Runtime configuration update"
	case OperationGarbageCollect:
		return "Garbage collection"
	case OperationReconfigure:
		return fmt.Sprintf("Advertise address change to %v",
			o.GetReconfigure().IP)
	}
	return "Unknown operation"
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
