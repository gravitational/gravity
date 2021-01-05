package phases

import (
	"context"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewConnect returns executor for the "connect" phase that connects
// installed cluster to an Ops Center
func NewConnect(p fsm.ExecutorParams, operator ossops.Operator) (*connectExecutor, error) {
	// the cluster should already be up at this point
	clusterOperator, err := environment.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &connectExecutor{
		FieldLogger:     logger,
		ClusterOperator: clusterOperator,
		ExecutorParams:  p,
	}, nil
}

type connectExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ClusterOperator is the ops client for the local gravity cluster
	ClusterOperator ops.Operator
	// ExecutorParams contains executor params
	fsm.ExecutorParams
}

// Execute connects cluster to an Ops Center
func (p *connectExecutor) Execute(ctx context.Context) error {
	trustedCluster, err := storage.UnmarshalTrustedCluster(
		p.Phase.Data.TrustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Progress.NextStep("Connecting to the Ops Center %v",
		trustedCluster.GetName())
	err = p.ClusterOperator.UpsertTrustedCluster(ossops.SiteKey{
		AccountID:  p.Plan.AccountID,
		SiteDomain: p.Plan.ClusterName,
	}, trustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Connected to the Ops Center %v.", trustedCluster.GetName())
	return nil
}

// Rollback is no-op for this phase
func (*connectExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure the phase is executed on a master node
func (p *connectExecutor) PreCheck(ctx context.Context) error {
	err := fsm.CheckMasterServer(p.Plan.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*connectExecutor) PostCheck(ctx context.Context) error {
	return nil
}
