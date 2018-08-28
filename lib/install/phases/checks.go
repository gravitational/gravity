package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewChecks creates a new preflight checks executor
func NewChecks(p fsm.ExecutorParams, operator ops.Operator, key ops.SiteOperationKey) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: log.WithField(constants.FieldInstallPhase, p.Phase.ID),
		Key:         opKey(p.Plan),
		Operator:    operator,
	}
	return &checksExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		operator:       operator,
		servers:        p.Plan.Servers,
		key:            key,
	}, nil
}

// PreCheck is no-op for this phase
func (r *checksExecutor) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (r *checksExecutor) PostCheck(context.Context) error {
	return nil
}

// Execute runs preflight checks
func (r *checksExecutor) Execute(ctx context.Context) error {
	r.Progress.NextStep("Running pre-flight checks")
	r.Info("Running pre-flight checks.")
	req := ops.ValidateServersRequest{
		AccountID:   r.key.AccountID,
		SiteDomain:  r.key.SiteDomain,
		OperationID: r.key.OperationID,
		Servers:     r.servers,
	}
	err := r.operator.ValidateServers(req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is a no-op for this phase
func (r *checksExecutor) Rollback(context.Context) error {
	return nil
}

// checksExecutor is the phase which executes preflight checks on a set of nodes
type checksExecutor struct {
	// FieldLogger specifies the logger used by the executor
	log.FieldLogger
	// ExecutorParams contains common executor parameters
	fsm.ExecutorParams
	key      ops.SiteOperationKey
	operator ops.Operator
	// servers is the list of local cluster servers
	servers []storage.Server
}
