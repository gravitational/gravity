package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/install/dispatcher"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/system/signals"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
)

// Engine implements the process of cluster installation
type Engine interface {
	// Execute executes the steps to install a cluster.
	// If the method returns with an error, the installer will continue
	// running until it receives a shutdown signal.
	//
	// The method is expected to be re-entrant as the service might be re-started
	// multiple times before the operation is complete.
	//
	// installer is the reference to the installer.
	// config specifies the configuration for the operation
	Execute(ctx context.Context, installer Interface, config Config) (dispatcher.Status, error)
}

// NotifyOperationAvailable is invoked by the engine to notify the server
// that the operation has been created.
// Implements Interface
func (i *Installer) NotifyOperationAvailable(op ops.SiteOperation) error {
	if err := i.startAgent(op); err != nil {
		return trace.Wrap(err)
	}
	i.registerExitHandlersForAgents(op)
	go func() {
		err := ProgressPoller{
			FieldLogger:  i.FieldLogger,
			Operator:     i.config.Operator,
			OperationKey: op.Key(),
			Dispatcher:   i.dispatcher,
		}.Run(i.ctx)
		if err != nil {
			i.Warnf("Failed in progress poller: %v.", err)
		}
	}()

	return nil
}

// NewCluster returns a new cluster create request.
// Implements engine.ClusterFactory
func (i *Installer) NewCluster() ops.NewSiteRequest {
	return i.config.ClusterFactory.NewCluster()
}

// ExecuteOperation executes the specified operation to completion.
// Implements Interface
func (i *Installer) ExecuteOperation(operationKey ops.SiteOperationKey) error {
	err := i.initOperationPlan(operationKey)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	machine, err := i.config.FSMFactory.NewFSM(i.config.Operator, operationKey)
	if err != nil {
		return trace.Wrap(err)
	}
	progressReporter := dispatcher.NewProgressReporter(i.ctx,
		i.dispatcher, "Executing operation")
	defer progressReporter.Stop()
	err = machine.ExecutePlan(i.ctx, progressReporter)
	if err != nil {
		i.WithError(err).Warn("Failed to execute operation plan.")
	}
	if completeErr := machine.Complete(i.ctx, err); completeErr != nil {
		i.WithError(completeErr).Warn("Failed to complete operation.")
		if err == nil {
			err = completeErr
		}
	}
	return trace.Wrap(err)
}

// CompleteOperation executes additional steps after the installation has completed.
// Implements Interface
func (i *Installer) CompleteOperation(operation ops.SiteOperation) error {
	var errors []error
	if err := i.uploadInstallLog(operation.Key()); err != nil {
		errors = append(errors, err)
	}
	if err := i.emitAuditEvents(operation); err != nil {
		errors = append(errors, err)
	}
	i.sendElapsedTime(operation.Created)
	return trace.NewAggregate(errors...)
}

// CompleteFinalInstallStep marks the final install step as completed unless
// the application has a custom install step - in which case it does nothing
// because it will be completed by user later.
// Implements Interface
func (i *Installer) CompleteFinalInstallStep(key ops.SiteOperationKey, delay time.Duration) error {
	if i.config.App.Manifest.SetupEndpoint() != nil {
		return nil
	}
	req := ops.CompleteFinalInstallStepRequest{
		AccountID:           key.AccountID,
		SiteDomain:          key.SiteDomain,
		WizardConnectionTTL: delay,
	}
	i.WithField("req", req).Debug("Completing final install step.")
	if err := i.config.Operator.CompleteFinalInstallStep(req); err != nil {
		return trace.Wrap(err, "failed to complete final install step")
	}
	return nil
}

// PrintStep publishes a progress entry described with (format, args) tuple to the client.
// Implements Interface
//nolint:goprintffuncname
func (i *Installer) PrintStep(format string, args ...interface{}) {
	event := dispatcher.Event{
		Progress: &ops.ProgressEntry{
			Message: fmt.Sprintf(format, args...),
		},
	}
	i.dispatcher.Send(event)
}

// wait blocks until either the context has been cancelled or the wizard process
// exits with an error.
func (i *Installer) wait() error {
	if err := i.runStoppers(i.ctx, i.completers); err != nil {
		i.WithError(err).Warn("Stoppers failed to run.")
	}
	return trace.Wrap(i.config.Process.Wait())
}

func (i *Installer) registerExitHandlersForAgents(op ops.SiteOperation) {
	i.addAborter(signals.StopperFunc(func(ctx context.Context) error {
		i.WithField("operation", op.ID).Info("Aborting agent service.")
		return trace.Wrap(i.config.Process.AgentService().AbortAgents(ctx, op.Key()))
	}))
	i.addStopper(signals.StopperFunc(func(ctx context.Context) error {
		i.WithField("operation", op.ID).Info("Stopping agent service.")
		return trace.Wrap(i.config.Process.AgentService().StopAgents(ctx, op.Key()))
	}))
	i.addCompleter(signals.StopperFunc(func(ctx context.Context) error {
		i.WithField("operation", op.ID).Info("Completing agent service.")
		return trace.Wrap(i.config.Process.AgentService().CompleteAgents(ctx, op.Key()))
	}))
}

func (i *Installer) sendElapsedTime(timeStarted time.Time) {
	event := dispatcher.Event{
		Progress: &ops.ProgressEntry{
			Message: color.GreenString("The operation has finished successfully in %v",
				time.Since(timeStarted).Truncate(time.Second)),
		},
	}
	i.dispatcher.Send(event)
}

// TODO(dmitri): this information should also be displayed when working with the operation
// manually
func (i *Installer) newCompletionEvent(status dispatcher.Status) *dispatcher.Event {
	var buf bytes.Buffer
	i.printEndpoints(&buf)
	if m, ok := modules.Get().(modules.Messager); ok {
		fmt.Fprintf(&buf, "\n%v", m.PostInstallMessage())
	}
	return &dispatcher.Event{
		Progress: &ops.ProgressEntry{
			Message:    buf.String(),
			Completion: constants.Completed,
		},
		// Set the completion status
		Status: status,
	}
}

func (i *Installer) runStoppers(ctx context.Context, stoppers []signals.Stopper) error {
	var errors []error
	for _, c := range stoppers {
		if err := c.Stop(ctx); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func (i *Installer) printEndpoints(w io.Writer) {
	status, err := i.getClusterStatus()
	if err != nil {
		i.WithError(err).Error("Failed to collect cluster status.")
		return
	}
	fmt.Fprintln(w)
	status.Cluster.Endpoints.Cluster.WriteTo(w) //nolint:errcheck
	fmt.Fprintln(w)
	status.Cluster.Endpoints.Applications.WriteTo(w) //nolint:errcheck
}

// getClusterStatus collects status of the installer cluster.
func (i *Installer) getClusterStatus() (*status.Status, error) {
	clusterOperator, err := localenv.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := clusterOperator.GetLocalSite(i.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	status, err := status.FromCluster(i.ctx, clusterOperator, *cluster, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return status, nil
}

// uploadInstallLog uploads user-facing operation log to the installed cluster
func (i *Installer) uploadInstallLog(operationKey ops.SiteOperationKey) error {
	file, err := os.Open(i.config.UserLogFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()
	err = i.config.Operator.StreamOperationLogs(operationKey, file)
	if err != nil {
		return trace.Wrap(err, "failed to upload install log")
	}
	i.WithField("file", i.config.UserLogFile).Debug("Uploaded install log to the cluster.")
	return nil
}

// emitAuditEvents sends the install operation's start/finish
// events to the installed cluster's audit log.
func (i *Installer) emitAuditEvents(operation ops.SiteOperation) error {
	operator, err := localenv.ClusterOperator()
	if err != nil {
		i.WithError(err).Warn("Failed to create cluster operator.")
		return trace.Wrap(err)
	}
	fields := events.FieldsForOperation(operation)
	events.Emit(i.ctx, operator, events.OperationInstallStart, fields.WithField(
		events.FieldTime, operation.Created))
	events.Emit(i.ctx, operator, events.OperationInstallComplete, fields)
	return nil
}

func (i *Installer) generateDebugReport(ctx context.Context, clusterKey ops.SiteKey, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(f.Name())
		}
	}()
	rc, err := i.config.Operator.GetSiteReport(ctx, ops.GetClusterReportRequest{SiteKey: clusterKey})
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer rc.Close()
	_, err = io.Copy(f, rc)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

func (i *Installer) addStopper(stopper signals.Stopper) {
	i.stoppers = append(i.stoppers, stopper)
}

func (i *Installer) addAborter(aborter signals.Stopper) {
	i.aborters = append(i.aborters, aborter)
}

func (i *Installer) addCompleter(completer signals.Stopper) {
	i.completers = append(i.completers, completer)
}
