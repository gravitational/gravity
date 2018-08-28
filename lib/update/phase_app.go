package update

import (
	"context"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// updatePhaseApp is the executor for the app update phase
type updatePhaseApp struct {
	log.FieldLogger
	phaseApp
}

// NewUpdatePhaseApp returns a new app phase executor
func NewUpdatePhaseApp(c FSMConfig, plan storage.OperationPlan, phase storage.OperationPhase) (*updatePhaseApp, error) {
	cluster, err := c.Operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if phase.Data.Package == nil {
		return nil, trace.NotFound("no package specified for phase %q", phase.ID)
	}
	return &updatePhaseApp{
		FieldLogger: log.NewEntry(log.New()),
		phaseApp: phaseApp{
			Apps:           c.Apps,
			Client:         c.Client,
			GravityPackage: plan.GravityPackage,
			Package:        *phase.Data.Package,
			Servers:        plan.Servers,
			ServiceUser:    cluster.ServiceUser,
		}}, nil
}

// Execute runs update/post-update hooks for the app
func (p *updatePhaseApp) Execute(ctx context.Context) error {
	var err error
	if p.Package.Name == constants.BootstrapConfigPackage {
		// the "bootstrap" app (rbac-app) is a special case as its resources contain
		// cluster roles and pod security policies which are needed for proper cluster
		// functioning so we create these resources directly because we may not have
		// permissions to launch jobs just yet
		err = p.createBootstrapResources()
	} else {
		err = p.runHooks(ctx, schema.HookUpdate, schema.HookUpdated)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback runs rollback/post-rollback hooks for the app
func (p *updatePhaseApp) Rollback(ctx context.Context) error {
	err := p.runHooks(ctx, schema.HookRollback, schema.HookRolledBack)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *updatePhaseApp) createBootstrapResources() error {
	reader, err := p.Apps.GetAppResources(p.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	err = utils.WithTempDir(func(dir string) error {
		err := dockerarchive.Untar(reader, dir, archive.DefaultOptions())
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(resources.ForEachObjectInFile(
			filepath.Join(dir, defaults.ResourcesDir, defaults.ResourcesFile),
			fsm.GetUpsertBootstrapResourceFunc(p.Client)))
	}, "resources")
	return trace.Wrap(err)
}

// updatePhaseBeforeApp is an executor for application's pre-update hook
type updatePhaseBeforeApp struct {
	log.FieldLogger
	phaseApp
}

// NewUpdatePhaseBeforeApp returns a new executor for running application pre-update hook
func NewUpdatePhaseBeforeApp(c FSMConfig, plan storage.OperationPlan, phase storage.OperationPhase) (*updatePhaseBeforeApp, error) {
	if phase.Data.Package == nil {
		return nil, trace.NotFound("no package specified for phase %q", phase.ID)
	}
	return &updatePhaseBeforeApp{
		FieldLogger: log.NewEntry(log.New()),
		phaseApp: phaseApp{
			Apps:           c.Apps,
			Client:         c.Client,
			GravityPackage: plan.GravityPackage,
			Package:        *phase.Data.Package,
			Servers:        plan.Servers,
		}}, nil
}

// Execute runs pre-update hook for the app
func (p *updatePhaseBeforeApp) Execute(ctx context.Context) error {
	err := p.runHooks(ctx, schema.HookBeforeUpdate)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is a no-op for this phase
func (p *updatePhaseBeforeApp) Rollback(context.Context) error {
	return nil
}

type phaseApp struct {
	// Apps is the cluster apps service
	Apps app.Applications
	// Client is the cluster Kubernetes client
	Client *kubernetes.Clientset
	// GravityPackage is the gravity binary package to run hooks with
	GravityPackage loc.Locator
	// Package is the package to run hooks for
	Package loc.Locator
	// Servers is the list of local cluster servers
	Servers []storage.Server
	// ServiceUser is the user used for services and system storage
	ServiceUser storage.OSUser
}

// PreCheck makes sure this phase is being executed on a master node
func (p *phaseApp) PreCheck(context.Context) error {
	return trace.Wrap(fsm.CheckMasterServer(p.Servers))
}

// PostCheck is no-op for this phase
func (p *phaseApp) PostCheck(context.Context) error {
	return nil
}

func (p *phaseApp) runHooks(ctx context.Context, hooks ...schema.HookType) error {
	for _, hook := range hooks {
		req := app.HookRunRequest{
			Application:    p.Package,
			GravityPackage: p.GravityPackage,
			Hook:           hook,
			Env: map[string]string{
				// TODO(r0mant) see if we can get rid of this flag
				constants.ManualUpdateEnvVar: "true",
			},
			ServiceUser: p.ServiceUser,
		}
		_, err := app.CheckHasAppHook(p.Apps, req)
		if err != nil {
			if trace.IsNotFound(err) {
				log.Debugf("%v does not have %v hook.", p.Package, hook)
				continue
			}
			return trace.Wrap(err)
		}
		_, out, err := app.RunAppHook(ctx, p.Apps, req)
		if err != nil {
			return trace.Wrap(err, "%v %s hook failed: %s", p.Package, hook, out)
		}
		log.Debugf("%v %s hook output: %s.", p.Package, hook, out)
	}
	return nil
}
