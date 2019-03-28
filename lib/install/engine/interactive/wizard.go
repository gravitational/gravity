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

	"github.com/gravitational/trace"
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
	install.Planner
	// Operator specifies the service operator
	ops.Operator
}

func (r *Engine) Execute(ctx context.Context, installer install.Installer) error {
	r.printURL(installer.AdvertiseAddr, installer.Printer)
	err := install.ExportRPCCredentials(ctx, installer.Packages, r.FieldLogger)
	if err != nil {
		return trace.Wrap(err, "failed to export RPC credentials")
	}
	installer.PrintStep("Waiting for the operation to start")
	operation, err := r.waitForOperation(ctx, r.Operator)
	if err != nil {
		return trace.Wrap(err, "failed to wait for operation to become ready")
	}
	if err := engine.ExecuteOperation(r.Planner, r.StateMachineFactory, r.Operator, operation.Key()); err != nil {
		return trace.Wrap(err)
	}
	installer.PrintPostInstallBanner()
	return wait(ctx, installer.Process, installer.Printer)
}

func (r *Engine) waitForOperation(ctx context.Context, operator ops.Operator) (operation *ops.SiteOperation, err error) {
	b := utils.NewUnlimitedExponentialBackOff()
	err := utils.RetryInterval(ctx, b, func() error {
		clusters, err := Operator.GetSites(defaults.SystemAccountID)
		if err != nil {
			return trace.Wrap(err, "failed to fetch clusters")
		}
		if len(clusters) == 0 {
			return trace.NotFound(err, "no clusters created yet")
		}
		cluster := clusters[0]
		operations, err := operator.GetSiteOperations(cluster.Key())
		if err != nil {
			return trace.Wrap(err, "failed to fetch operations")
		}
		if len(operations) == 0 {
			return trace.NotFound(err, "no operations created yet")
		}
		operation = operations[0]
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
func (r *Engine) printURL(advertiseAddr string, printer utils.Printer) {
	printer.PrintStep("Starting web UI install wizard")
	url := fmt.Sprintf("https://%v/web/installer/new/%v/%v/%v?install_token=%v",
		advertiseAddr,
		r.App.Package.Repository,
		r.App.Package.Name,
		r.App.Package.Version,
		r.Token.Token)
	r.WithField("installer-url", url).Info("Generated installer URL.")
	rule := strings.Repeat("-", 100)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%v\n", rule)
	fmt.Fprintf(&buf, "%v\n", rule)
	fmt.Fprintf(&buf, "OPEN THIS IN BROWSER: %v\n", url)
	fmt.Fprintf(&buf, "%v\n", rule)
	fmt.Fprintf(&buf, "%v\n", rule)
	printer.PrintStep(buf.String())
}

func wait(ctx context.Context, p process.GravityProcess, printer utils.Printer) error {
	printer.Print("\nInstaller process will keep running so the installation can be finished by\n" +
		"completing necessary post-install actions in the installer UI if the installed\n" +
		"application requires it.\n" +
		color.YellowString("\nOnce no longer needed, press Ctrl-C to shutdown this process.\n"),
	)
	errC := make(chan error, 1)
	go func() {
		errC <- p.Wait()
	}()
	select {
	case err := <-errC:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}
