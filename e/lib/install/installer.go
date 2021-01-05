// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import (
	"github.com/gravitational/gravity/lib/fsm"
	ossinstall "github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/engine"
	"github.com/gravitational/gravity/lib/ops"
)

// Config defines the installer configuration
type Config struct {
	// Config is the base installer configuration
	ossinstall.Config
	// License is optional license string
	License string
	// RemoteOpsURL is the URL of the remote Ops Center
	RemoteOpsURL string
	// RemoteOpsToken is the remote Ops Center auth token
	RemoteOpsToken string
	// OperationID is the ID of the operation when installing via Ops Center
	OperationID string
	// OpsTunnelToken is the token used when creating a trusted cluster
	OpsTunnelToken string
	// OpsSNIHost is the Ops Center SNI host
	OpsSNIHost string
	// OpsAdvertiseAddr is the Ops Center advertise address
	OpsAdvertiseAddr string
}

// CheckAndSetDefaults checks the parameters and sets some defaults
func (c *Config) CheckAndSetDefaults() error {
	return nil
}

// NewFSMFactory returns a new state machine factory
func NewFSMFactory(config Config) engine.FSMFactory {
	return &fsmFactory{
		Config: config,
	}
}

// NewStateMachine creates a new state machine for the specified operator and operation.
// Implements engine.FSMFactory
func (r *fsmFactory) NewFSM(operator ops.Operator, operationKey ops.SiteOperationKey) (fsm *fsm.FSM, err error) {
	config := ossinstall.NewFSMConfig(operator, operationKey, r.Config.Config)
	config.Spec = FSMSpec(config)
	return ossinstall.NewFSM(config)
}

type fsmFactory struct {
	Config
}

// NewClusterFactory returns a factory for creating cluster requests
func NewClusterFactory(config Config, base engine.ClusterFactory) engine.ClusterFactory {
	return &clusterFactory{
		Config: config,
		base:   base,
	}
}

// NewCluster creates the cluster with the specified operator
// Implements engine.ClusterFactory
func (r *clusterFactory) NewCluster() ops.NewSiteRequest {
	req := r.base.NewCluster()
	req.License = r.License
	return req
}

type clusterFactory struct {
	Config
	base engine.ClusterFactory
}
