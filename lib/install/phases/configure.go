package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewConfigure returns a new "configure" phase executor
func NewConfigure(p fsm.ExecutorParams, operator ops.Operator) (*configureExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldInstallPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
	}
	return &configureExecutor{
		FieldLogger:    logger,
		Operator:       operator,
		ExecutorParams: p,
	}, nil
}

type configureExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Operator is the installer process ops service
	Operator ops.Operator
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the configure phase
func (p *configureExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Configuring cluster packages")
	p.Info("Configuring cluster packages.")
	err := p.Operator.ConfigurePackages(ops.SiteOperationKey{
		AccountID:   p.Plan.AccountID,
		SiteDomain:  p.Plan.ClusterName,
		OperationID: p.Plan.OperationID,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is no-op for this phase
func (*configureExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*configureExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*configureExecutor) PostCheck(ctx context.Context) error {
	return nil
}
