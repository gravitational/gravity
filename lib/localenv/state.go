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

package localenv

import (
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// LocalState describes the local state of the cluster as represented by
// the information found in the node-local backend.
type LocalState struct {
	// Cluster is the cluster that's installed on the node.
	Cluster storage.Site
	// InstallOperation is the original install operation.
	InstallOperation storage.SiteOperation
}

// Server returns the server from the state.
//
// The method currenly expects the state to contain only 1 server and returns
// an error otherwise.
func (s LocalState) Server() (*storage.Server, error) {
	servers := s.Cluster.ClusterState.Servers
	if len(servers) != 1 {
		return nil, trace.BadParameter("expected 1 server, got: %s", servers)
	}
	return &servers[0], nil
}

// GetLocalState returns cluster state from the local node backend.
func (env *LocalEnvironment) GetLocalState() (*LocalState, error) {
	cluster, err := env.Backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operations, err := storage.GetOperations(env.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, operation := range operations {
		if operation.Type == ops.OperationInstall {
			return &LocalState{
				Cluster:          *cluster,
				InstallOperation: operation,
			}, nil
		}
	}
	return nil, trace.NotFound("no install operation found in the local state")
}
