package install

import (
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// initOperationPlan initializes the install operation plan and saves it
// into the installer database
func (i *Installer) initOperationPlan() error {
	clusters, err := i.Operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(clusters) != 1 {
		return trace.BadParameter("expected 1 cluster, got: %v", clusters)
	}
	op, _, err := ops.GetInstallOperation(clusters[0].Key(), i.Operator)
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := i.Operator.GetOperationPlan(op.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if plan != nil {
		return trace.AlreadyExists("plan is already initialized")
	}
	plan, err = i.engine.GetOperationPlan(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	err = i.Operator.CreateOperationPlan(op.Key(), *plan)
	if err != nil {
		return trace.Wrap(err)
	}
	i.Info("Initialized operation plan.")
	return nil
}

// GetOperationPlan builds a plan for the provided operation
func (i *Installer) GetOperationPlan(op ops.SiteOperation) (*storage.OperationPlan, error) {
	builder, err := i.GetPlanBuilder(op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan := &storage.OperationPlan{
		OperationID:   op.ID,
		OperationType: op.Type,
		AccountID:     op.AccountID,
		ClusterName:   op.SiteDomain,
		Servers:       append(builder.Masters, builder.Nodes...),
	}

	switch i.Mode {
	case constants.InstallModeCLI:
		builder.AddChecksPhase(plan)
	}

	// configure packages for all nodes
	builder.AddConfigurePhase(plan)

	// bootstrap each node: setup directories, users, etc.
	builder.AddBootstrapPhase(plan)

	// pull configured packages on each node
	builder.AddPullPhase(plan)

	// install system software on master nodes
	builder.AddMastersPhase(plan)

	// (optional) install system software on regular nodes
	if len(builder.Nodes) > 0 {
		builder.AddNodesPhase(plan)
	}

	// perform post system install tasks such as waiting for planet
	// to start up, labeling and tainting nodes, etc.
	builder.AddWaitPhase(plan)
	builder.AddLabelPhase(plan)
	builder.AddRBACPhase(plan)

	// if installing a regular app, the resources might have been
	// provided by a user
	if len(i.Cluster.Resources) != 0 {
		builder.AddResourcesPhase(plan, i.Cluster.Resources)
	}

	// export applications to registries
	builder.AddExportPhase(plan)

	// install runtime application
	err = builder.AddRuntimePhase(plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// install user application
	err = builder.AddApplicationPhase(plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// re-enable planet leader elections
	builder.AddEnableElectionPhase(plan)

	return plan, nil
}
