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

package reconfigure

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"

	"github.com/gravitational/trace"
)

// State represents the local state of the cluster/node at the start of the
// reconfiguration operation.
type State struct {
	// Cluster is the cluster that's installed on the node.
	Cluster storage.Site
	// InstallOperation is the original install operation.
	InstallOperation storage.SiteOperation
}

// Server returns the server from the state.
func (s State) Server() (*storage.Server, error) {
	servers := s.Cluster.ClusterState.Servers
	if len(servers) != 1 {
		return nil, trace.BadParameter("expected 1 server, got: %#v", servers)
	}
	return &servers[0], nil
}

// GetLocalState returns the state of the cluster installed on the reconfigure node.
//
// As there is no cluster running on the node before the reconfigure operation
// (since all services are supposed to be stopped), we use packages in the node
// local state, specifically site-export package, to obtain this information.
func GetLocalState(packages pack.PackageService) (*State, error) {
	exportPackage, err := pack.FindPackage(packages, func(e pack.PackageEnvelope) bool {
		return e.Locator.Name == constants.SiteExportPackage
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, reader, err := packages.ReadPackage(exportPackage.Locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	tempFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer os.Remove(tempFile.Name())
	_, err = io.Copy(tempFile, reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	backend, err := keyval.NewBolt(keyval.BoltConfig{Path: tempFile.Name()})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer backend.Close()
	cluster, err := backend.GetSite(exportPackage.Locator.Repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operations, err := storage.GetOperations(backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, operation := range operations {
		if operation.Type == ops.OperationInstall {
			return &State{
				Cluster:          *cluster,
				InstallOperation: operation,
			}, nil
		}
	}
	return nil, trace.NotFound("install operation not found")
}

// ValidateLocalState executes a few sanity checks on the state of the local
// cluster (collected by the function above) to make sure that it can be
// reconfigured.
func ValidateLocalState(state *State) error {
	// Only single-node clusters are supported.
	// TODO(r0mant): Because the above function uses the original site-export
	// package to build the cluster state, this check can either fail if a
	// multi-node cluster was shrunk down to 1 node, or will let the operation
	// through if a 1-node cluster was expanded. Need some way to deduce the
	// most recent cluster state based on the local state.
	if len(state.Cluster.ClusterState.Servers) != 1 {
		return trace.BadParameter("reconfiguration is only supported for single-node clusters, this one appears to have %v nodes",
			len(state.Cluster.ClusterState.Servers))
	}
	return nil
}
