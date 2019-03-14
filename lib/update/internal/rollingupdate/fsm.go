package rollingupdate

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/update"

	"github.com/gravitational/trace"
	"k8s.io/client-go/kubernetes"
)

// NewMachine creates an operation FSM that implements a rolling update strategy
func NewMachine(ctx context.Context, config Config) (*fsm.FSM, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	engine, err := update.NewEngine(ctx, config.Config, &updateDispatcher{
		Config:     config,
		Dispatcher: config.Dispatcher,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	machine, err := update.NewMachine(ctx, config.Config, engine)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return machine, nil
}

func (r *Config) checkAndSetDefaults() error {
	if err := r.Config.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.Apps == nil {
		return trace.BadParameter("cluster application service is required")
	}
	if r.ClusterPackages == nil {
		return trace.BadParameter("cluster package service is required")
	}
	if r.HostLocalPackages == nil {
		return trace.BadParameter("host-local package service is required")
	}
	if r.Dispatcher == nil {
		r.Dispatcher = NewDefaultDispatcher()
	}
	return nil
}

// Config describes configuration for executing a rolling update operation
type Config struct {
	update.Config
	// Dispatcher specifies optional phase dispatcher.
	// If unspecified, default dispatcher is used.
	//
	// Implementations that reimplement certain steps or implement new steps
	// can set this field and use an instance of the default dispatcher as a fallback
	Dispatcher
	// HostLocalPackages specifies the package service on local host
	HostLocalPackages update.LocalPackageService
	// Apps is the cluster application service
	Apps app.Applications
	// ClusterPackages specifies the cluster package service
	ClusterPackages pack.PackageService
	// Client specifies the optional kubernetes client
	Client *kubernetes.Clientset
}
