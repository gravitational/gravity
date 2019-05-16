package engine

import (
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/fatih/color"
)

// StateMachineFactory creates installer state machines
type StateMachineFactory interface {
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
