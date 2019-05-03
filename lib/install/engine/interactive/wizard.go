package interactive

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install"
	libengine "github.com/gravitational/gravity/lib/install/engine"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New returns a new installer that implements interactive installation
// workflow
func New(config Config) (*Engine, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Engine{
		Config: config,
	}, nil
}

func (r *Config) checkAndSetDefaults() error {
	if r.FieldLogger == nil {
		return trace.BadParameter("FieldLogger is required")
	}
	if r.StateMachineFactory == nil {
		return trace.BadParameter("StateMachineFactory is required")
	}
	if r.Planner == nil {
		return trace.BadParameter("Planner is required")
	}
	if r.Operator == nil {
		return trace.BadParameter("Operator is required")
	}
	return nil
}

// Config defines the installer configuration
type Config struct {
	// FieldLogger is the logger for the installer
	log.FieldLogger
	// StateMachineFactory is a factory for creating installer state machines
	libengine.StateMachineFactory
	// Planner creates a plan for the operation
	libengine.Planner
	// Operator specifies the service operator
	ops.Operator
	// AdvertiseAddr specifies the advertise address of the wizard
	AdvertiseAddr string
}

// Validate is a no-op for this engine
func (r *Engine) Validate(context.Context, install.Config) (err error) {
	return nil
}

// Execute runs the wizard operation
func (r *Engine) Execute(ctx context.Context, installer install.Interface, config install.Config) error {
	e, err := newExecutor(ctx, r, installer, config)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := e.bootstrap(); err != nil {
		return trace.Wrap(err)
	}
	e.printURL()
	installer.PrintStep("Waiting for the operation to start")
	operation, err := e.waitForOperation()
	if err != nil {
		return trace.Wrap(err, "failed to wait for operation to become ready")
	}
	if err := installer.NotifyOperationAvailable(operation.Key()); err != nil {
		return trace.Wrap(err)
	}
	if err := e.executeOperation(operation.Key()); err != nil {
		return trace.Wrap(err)
	}
	if err := e.finalizeOperation(*operation); err != nil {
		return trace.Wrap(err)
	}
	// TODO(dmitri): this should not be necessary if there's a way to send the completion notification
	// from bandwagon to installer
	installer.PrintStep("\nInstaller process will keep running so the installation can be finished by\n" +
		"completing necessary post-install actions in the installer UI if the installed\n" +
		"application requires it.\n" +
		color.YellowString("\nOnce no longer needed, press Ctrl-C to shutdown this process.\n"),
	)
	return trace.Wrap(installer.Wait())
}

func (r *executor) bootstrap() error {
	// Extract RPC credentials for the agent service to be able to accept
	// and control remote agents
	err := install.ExportRPCCredentials(r.ctx, r.config.Packages, r.FieldLogger)
	if err != nil {
		return trace.Wrap(err, "failed to export RPC credentials")
	}
	return nil
}

func (r *executor) waitForOperation() (operation *ops.SiteOperation, err error) {
	b := utils.NewUnlimitedExponentialBackOff()
	err = utils.RetryWithInterval(r.ctx, b, func() error {
		clusters, err := r.Operator.GetSites(defaults.SystemAccountID)
		if err != nil {
			return trace.Wrap(err, "failed to fetch clusters")
		}
		if len(clusters) == 0 {
			return trace.NotFound("no clusters created yet")
		}
		cluster := clusters[0]
		operations, err := r.Operator.GetSiteOperations(cluster.Key())
		if err != nil {
			return trace.Wrap(err, "failed to fetch operations")
		}
		if len(operations) == 0 {
			return trace.NotFound("no operations created yet")
		}
		operation = (*ops.SiteOperation)(&operations[0])
		r.WithField("operation", operation.Key()).Info("Fetched operation.")
		if operation.State != ops.OperationStateReady {
			return trace.BadParameter("operation is not ready")
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operation, nil
}

func (r *executor) executeOperation(operationKey ops.SiteOperationKey) error {
	return trace.Wrap(libengine.ExecuteOperation(r.ctx, r.Planner, r.StateMachineFactory,
		r.Operator, operationKey, r.FieldLogger))
}

func (r *executor) finalizeOperation(operation ops.SiteOperation) error {
	// With an interactive installation, the link to remote Ops Center cannot be removed
	// immediately as it is used to tunnel final install step
	if r.app.Manifest.SetupEndpoint() == nil {
		if err := r.CompleteFinalInstallStep(operation.Key(), defaults.WizardLinkTTL); err != nil {
			r.WithError(err).Warn("Failed to complete final install step.")
		}
	}
	if err := r.Finalize(operation); err != nil {
		r.WithError(err).Warn("Failed to finalize install.")
	}
	return nil
}

// printURL prints the URL that installer can be reached at via browser
// in interactive mode to stdout
func (r *executor) printURL() {
	r.PrintStep("Starting web UI install wizard")
	url := fmt.Sprintf("https://%v/web/installer/new/%v/%v/%v?install_token=%v",
		r.AdvertiseAddr,
		r.config.AppPackage.Repository,
		r.config.AppPackage.Name,
		r.config.AppPackage.Version,
		r.config.Token.Token)
	r.WithField("installer-url", url).Info("Generated installer URL.")
	ruler := strings.Repeat("-", 100)
	var buf bytes.Buffer
	fmt.Fprintln(&buf, ruler, "\n", ruler)
	fmt.Fprintln(&buf, "OPEN THIS IN BROWSER:", url)
	fmt.Fprintln(&buf, ruler, "\n", ruler)
	r.PrintStep(buf.String())
}

// Engine implements interactive installation workflow
type Engine struct {
	// Config specifies the engine's configuration
	Config
}

func newExecutor(ctx context.Context, r *Engine, installer install.Interface, config install.Config) (*executor, error) {
	app, err := config.GetApp()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query application")
	}
	return &executor{
		Config:    r.Config,
		Interface: installer,
		app:       *app,
		ctx:       ctx,
		config:    config,
	}, nil
}

type executor struct {
	Config
	app app.Application
	install.Interface
	config install.Config
	ctx    context.Context
}
