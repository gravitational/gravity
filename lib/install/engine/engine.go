/*
Copyright 2019 Gravitational, Inc.

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
package engine

import (
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/fatih/color"
)

// FSMFactory creates installer state machines
type FSMFactory interface {
	// NewFSM creates a new instance of installer state machine
	// using the specified operation key
	NewFSM(ops.Operator, ops.SiteOperationKey) (*fsm.FSM, error)
}

// ClusterFactory creates clusters
type ClusterFactory interface {
	// NewCluster returns a new request to create a cluster.
	// Returns the created cluster record
	NewCluster() ops.NewSiteRequest
}

// Planner constructs a plan for the install operation
type Planner interface {
	// GetOperationPlan returns a new plan for the install operation
	GetOperationPlan(ops.Operator, ops.Site, ops.SiteOperation) (*storage.OperationPlan, error)
}

func init() {
	// Enable color in progress step messages
	color.NoColor = false
}
