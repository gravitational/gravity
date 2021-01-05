package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/client"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/webpack"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewInstaller returns a new "installer" phase
func NewInstaller(p fsm.ExecutorParams, operator ops.Operator, wizardPack pack.PackageService, wizardApps app.Applications) (*installerExecutor, error) {
	// TODO pass insecure flag
	httpClient := httplib.GetClient(true)
	opsPack, err := webpack.NewBearerClient(p.Phase.Data.Agent.OpsCenterURL,
		p.Phase.Data.Agent.Password, roundtrip.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opsApps, err := client.NewBearerClient(p.Phase.Data.Agent.OpsCenterURL,
		p.Phase.Data.Agent.Password, client.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
	}
	return &installerExecutor{
		FieldLogger:    logger,
		WizardPackages: wizardPack,
		WizardApps:     wizardApps,
		OpsPackages:    opsPack,
		OpsApps:        opsApps,
		ExecutorParams: p,
	}, nil
}

type installerExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// WizardPackages is the installer process pack service
	WizardPackages pack.PackageService
	// WizardApps is the installer process app service
	WizardApps app.Applications
	// OpsPackages is the remote Ops Center pack service
	OpsPackages pack.PackageService
	// OpsApps is the remote Ops Center app service
	OpsApps app.Applications
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the installer phase
func (p *installerExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Downloading application installer from Ops Center")
	p.Info("Downloading application installer from Ops Center")
	puller := app.Puller{
		FieldLogger: p.FieldLogger,
		SrcPack:     p.OpsPackages,
		DstPack:     p.WizardPackages,
		SrcApp:      p.OpsApps,
		DstApp:      p.WizardApps,
		Upsert:      true,
	}
	err := puller.PullApp(ctx, *p.Phase.Data.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Info("Installer has been downloaded.")
	return nil
}

// Rollback is no-op for this phase
func (*installerExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*installerExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*installerExecutor) PostCheck(ctx context.Context) error {
	return nil
}
