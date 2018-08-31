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
	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"

	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
)

// FSMConfig is the expand FSM configuration
type FSMConfig struct {
	// Operator is operator of the cluster the node is joining to
	Operator ops.Operator
	// Apps is apps service of the cluster the node is joining to
	Apps app.Applications
	// Packages is package service of the cluster the node is joining to
	Packages pack.PackageService
	// LocalBackend is local backend of the joining node
	LocalBackend storage.Backend
	// LocalApps is local apps service of the joining node
	LocalApps app.Appliations
	// LocalPackages is local package service of the joining node
	LocalPackages pack.PackageService
	// Etcd is client to the cluster's etcd members API
	Etcd etcd.MembersAPI
}

// CheckAndSetDefaults validates expand FSM configuration and sets defaults
func (c *FSMConfig) CheckAndSetDefaults() error {
	if c.Operator == nil {
		return trace.BadParameter("missing Operator")
	}
	if c.Apps == nil {
		return trace.BadParameter("missing Apps")
	}
	if c.LocalBackend == nil {
		return trace.BadParameter("missing LocalBackend")
	}
	return nil
}

// NewFSM returns a new state machine for expand operation
func NewFSM(config FSMConfig) (*fsm.FSM, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &fsm.FSM{}, nil
}
