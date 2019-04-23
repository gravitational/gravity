package engine

import (
	"context"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// StateMachineFactory creates installer state machines
type StateMachineFactory interface {
	// NewStateMachine creates a new instance of installer state machine
	// using the specified operation key
	NewStateMachine(ops.Operator, ops.SiteOperationKey) (*fsm.FSM, error)
}

// ClusterFactory creates clusters
type ClusterFactory interface {
	// NewCluster returns a new request to create a cluster.
	// Returns the created cluster record
	NewCluster() ops.NewSiteRequest
}

// Planer constructs a plan for the install operation
type Planner interface {
	// GetOperationPlan returns a new plan for the install operation
	GetOperationPlan(ops.Site, ops.SiteOperation) (*storage.OperationPlan, error)
}

// ExecuteOperation executes the operation specified with the given key.
// It will initialize an operation plan if none has been created yet
func ExecuteOperation(
	ctx context.Context,
	planner Planner,
	fsmFactory StateMachineFactory,
	operator ops.Operator,
	operationKey ops.SiteOperationKey,
	logger log.FieldLogger,
) error {
	err := InitOperationPlan(operator, planner)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	machine, err := fsmFactory.NewStateMachine(operator, operationKey)
	if err != nil {
		return trace.Wrap(err)
	}
	planErr := machine.ExecutePlan(ctx, utils.DiscardProgress)
	if planErr != nil {
		logger.WithError(planErr).Warn("Failed to execute operation plan.")
	}
	if err := machine.Complete(planErr); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(planErr)
}

// InitOperationPlan initializes a new operation plan for the specified install operation
// in the given operator
func InitOperationPlan(operator ops.Operator, planner Planner) error {
	clusters, err := operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(clusters) != 1 {
		return trace.BadParameter("expected 1 cluster, got: %v", clusters)
	}
	operation, _, err := ops.GetInstallOperation(clusters[0].Key(), operator)
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := operator.GetOperationPlan(operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if plan != nil {
		return trace.AlreadyExists("plan is already initialized")
	}
	plan, err = planner.GetOperationPlan(clusters[0], *operation)
	if err != nil {
		return trace.Wrap(err)
	}
	err = operator.CreateOperationPlan(operation.Key(), *plan)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
