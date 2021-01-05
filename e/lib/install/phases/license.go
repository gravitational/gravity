package phases

import (
	"context"

	"github.com/gravitational/gravity/e/lib/ops/service"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// NewLicense returns a new "license" phase executor
func NewLicense(p fsm.ExecutorParams, operator ops.Operator, client *kubernetes.Clientset) (*licenseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &licenseExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Client:         client,
	}, nil
}

type licenseExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	// Client is the Kubernetes client
	Client *kubernetes.Clientset
}

// Execute executes the license phase
func (p *licenseExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Installing cluster license")
	err := service.InstallLicenseSecret(p.Client, string(p.Phase.Data.License))
	if err != nil {
		return trace.Wrap(err, "failed to install cluster license")
	}
	p.Info("Installed cluster license.")
	return nil
}

// Rollback is no-op for this phase
func (*licenseExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure this phase is executed on a master node
func (p *licenseExecutor) PreCheck(ctx context.Context) error {
	err := fsm.CheckMasterServer(p.Plan.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*licenseExecutor) PostCheck(ctx context.Context) error {
	return nil
}
