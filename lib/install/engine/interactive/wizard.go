package interactive

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/engine"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

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
	engine.StateMachineFactory
	// Planner creates a plan for the operation
	engine.Planner
	// Operator specifies the service operator
	ops.Operator
}

func (r *Engine) Execute(ctx context.Context, installer install.Interface, config install.RuntimeConfig) error {
	r.printURL(ctx, installer, config)
	err := install.ExportRPCCredentials(ctx, config.Packages, r.FieldLogger)
	if err != nil {
		return trace.Wrap(err, "failed to export RPC credentials")
	}
	installer.PrintStep(ctx, "Waiting for the operation to start")
	operation, err := r.waitForOperation(ctx, r.Operator)
	if err != nil {
		return trace.Wrap(err, "failed to wait for operation to become ready")
	}
	installer.AddAgentServiceCloser(ctx, operation.Key())
	if err := engine.ExecuteOperation(ctx, r.Planner, r.StateMachineFactory,
		r.Operator, operation.Key(), r.FieldLogger); err != nil {
		return trace.Wrap(err)
	}
	// With an interactive installation, the link to remote Ops Center cannot be removed
	// immediately as it is used to tunnel final install step
	if err := installer.CompleteFinalInstallStep(defaults.WizardLinkTTL); err != nil {
		r.WithError(err).Warn("Failed to complete final install step.")
	}
	if err := installer.Finalize(ctx, *operation); err != nil {
		r.WithError(err).Warn("Failed to finalize install.")
	}
	return nil
}

func (r *Engine) waitForOperation(ctx context.Context, operator ops.Operator) (operation *ops.SiteOperation, err error) {
	b := utils.NewUnlimitedExponentialBackOff()
	err = utils.RetryWithInterval(ctx, b, func() error {
		clusters, err := r.Operator.GetSites(defaults.SystemAccountID)
		if err != nil {
			return trace.Wrap(err, "failed to fetch clusters")
		}
		if len(clusters) == 0 {
			return trace.NotFound("no clusters created yet")
		}
		cluster := clusters[0]
		operations, err := operator.GetSiteOperations(cluster.Key())
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

// printURL prints the URL that installer can be reached at via browser
// in interactive mode to stdout
func (r *Engine) printURL(ctx context.Context, installer install.Interface, config install.RuntimeConfig) {
	installer.PrintStep(ctx, "Starting web UI install wizard")
	url := fmt.Sprintf("https://%v/web/installer/new/%v/%v/%v?install_token=%v",
		config.AdvertiseAddr,
		config.App.Package.Repository,
		config.App.Package.Name,
		config.App.Package.Version,
		config.Token)
	r.WithField("installer-url", url).Info("Generated installer URL.")
	ruler := strings.Repeat("-", 100)
	var buf bytes.Buffer
	fmt.Fprintln(&buf, ruler, "\n", ruler)
	fmt.Fprintln(&buf, "OPEN THIS IN BROWSER:", url)
	fmt.Fprintln(&buf, ruler, "\n", ruler)
	installer.PrintStep(ctx, buf.String())
}

type Engine struct {
	Config
}
