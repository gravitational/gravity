package update

import (
	"context"
	"fmt"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// updatePhaseNode is the executor for the update master/node update phase
type updatePhaseSystem struct {
	// OperationID is the id of the current update operation
	OperationID string
	// Server is the server currently being updated
	Server storage.Server
	// GravityPath is the path to the new gravity binary
	GravityPath string
	// FieldLogger is used for logging
	log.FieldLogger
	remote fsm.Remote
	// runtimePackage specifies the runtime package to update to
	runtimePackage loc.Locator
}

// NewUpdatePhaseNode returns a new node update phase executor
func NewUpdatePhaseSystem(c FSMConfig, plan storage.OperationPlan, phase storage.OperationPhase, remote fsm.Remote) (*updatePhaseSystem, error) {
	if phase.Data == nil || phase.Data.Server == nil {
		return nil, trace.NotFound("no server specified for phase %q", phase.ID)
	}
	if phase.Data.Package == nil {
		return nil, trace.NotFound("no application package specified for phase %q", phase.ID)
	}
	gravityPath, err := getGravityPath()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := c.Apps.GetApp(*phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runtimePackage, err := app.Manifest.RuntimePackageForProfile(phase.Data.Server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &updatePhaseSystem{
		OperationID:    plan.OperationID,
		Server:         *phase.Data.Server,
		GravityPath:    gravityPath,
		FieldLogger:    log.NewEntry(log.New()),
		remote:         remote,
		runtimePackage: *runtimePackage,
	}, nil
}

// PreCheck makes sure the phase is being executed on the correct server
func (p *updatePhaseSystem) PreCheck(ctx context.Context) error {
	return trace.Wrap(p.remote.CheckServer(ctx, p.Server))
}

// PostCheck is no-op for this phase
func (p *updatePhaseSystem) PostCheck(context.Context) error {
	return nil
}

// Execute runs system update on the node
func (p *updatePhaseSystem) Execute(context.Context) error {
	out, err := fsm.RunCommand([]string{p.GravityPath,
		"--insecure", "--debug", "system", "update",
		"--changeset-id", p.OperationID,
		"--runtime-package", p.runtimePackage.String(),
		"--with-status",
	})
	if err != nil {
		message := "failed to update system"
		if errUninstall, ok := trace.Unwrap(err).(*utils.ErrorUninstallService); ok {
			message = fmt.Sprintf("The %q service failed to stop."+
				"Restart this node to clean up and retry %q.",
				errUninstall.Package, updateSystem)
		}
		return trace.Wrap(err, message)
	}
	log.Infof("System updated: %s.", out)
	return nil
}

// Rollback runs rolls back the system upgrade on the node
func (p *updatePhaseSystem) Rollback(context.Context) error {
	out, err := fsm.RunCommand([]string{p.GravityPath, "--insecure", "system", "rollback",
		"--changeset-id", p.OperationID, "--with-status"})
	if err != nil {
		return trace.Wrap(err, "failed to rollback system: %s", out)
	}
	log.Infof("System rolled back: %s.", out)
	return nil
}
