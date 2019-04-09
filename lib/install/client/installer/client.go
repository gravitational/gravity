package installer

import (
	"context"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/gravitational/gravity/lib/defaults"
	libclient "github.com/gravitational/gravity/lib/install/client"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New returns a new client to handle the installer case.
// See docs/design/client/install.puml for details
func New(ctx context.Context, config Config) (*client, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c := &client{Config: config}
	err = c.checkLocalState()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = c.installSelfAsService()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err == nil {
			return
		}
		uninstallService()
	}()
	if config.ConnectTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.ConnectTimeout)
		defer cancel()
	}
	cc, err := installpb.NewClient(ctx, config.StateDir, config.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.client = cc
	c.addTerminationHandler(ctx)
	return c, nil
}

// Run starts the service operation and runs the loop to fetch and display
// operation progress
func (r *client) Run(ctx context.Context) error {
	stream, err := r.client.Execute(ctx, &installpb.ExecuteRequest{})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.runProgressLoop(ctx, stream))
}

func (r *Config) checkAndSetDefaults() error {
	if r.StateDir == "" {
		return trace.BadParameter("StateDir is required")
	}
	if r.ApplicationDir == "" {
		return trace.BadParameter("ApplicationDir is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("Packages is required")
	}
	if r.TermC == nil {
		return trace.BadParameter("TermC is required")
	}
	if r.Printer == nil {
		r.Printer = utils.DiscardPrinter
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:installer")
	}
	return nil
}

type Config struct {
	log.FieldLogger
	utils.Printer
	// StateDir specifies the install state directory on local host
	StateDir string
	// ApplicationDir specifies the read-only installer state directory
	ApplicationDir string
	// Packages specifies the host-local package service
	Packages pack.PackageService
	// TermC specifies the termination handler registration channel
	TermC chan<- utils.Stopper
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection. Wait forever, if unspecified
	ConnectTimeout time.Duration
}

// installSelfAsService installs a systemd unit using the current process's command line
// and turns on service mode
func (r *client) installSelfAsService() error {
	args := os.Args[1:]
	args = append(args,
		"--from-service",
		r.ApplicationDir,
	)
	return trace.Wrap(systemservice.ReinstallOneshotService(libclient.ServiceName, args...))
}

// checkLocalState performs a local environment sanity check to make sure
// that install/join on this node can proceed without issues
func (r *client) checkLocalState() error {
	// make sure that there are no packages in the local state left from
	// some improperly cleaned up installation
	packages, err := r.Packages.GetPackages(defaults.SystemAccountOrg)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(packages) != 0 {
		return trace.BadParameter("detected previous installation state in %v, "+
			"please clean it up using `gravity leave --force` before proceeding "+
			"(see https://gravitational.com/gravity/docs/cluster/#deleting-a-cluster for more details)",
			r.StateDir)
	}
	return nil
}

func (r *client) runProgressLoop(ctx context.Context, stream installpb.Agent_ExecuteClient) error {
	var resp installpb.ProgressResponse
	for !resp.Complete && !isDone(ctx.Done()) {
		resp, err := stream.Recv()
		if err != nil {
			r.WithError(err).Warn("Failed to fetch progress.")
			return trace.Wrap(err)
		}
		if len(resp.Errors) != 0 {
			r.PrintStep(color.RedString(resp.Errors[0].Message))
			continue
		}
		r.PrintStep(resp.Message)
	}
	return nil
}

func (r *client) addTerminationHandler(ctx context.Context) {
	select {
	case r.TermC <- utils.StopperFunc(func(ctx context.Context) error {
		_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
		return trace.Wrap(err)
	}):
	case <-ctx.Done():
	}
}

type client struct {
	Config
	client installpb.AgentClient
}

func uninstallService() error {
	return trace.Wrap(systemservice.UninstallService(libclient.ServiceName))
}

func isDone(doneC <-chan struct{}) bool {
	select {
	case <-doneC:
		return true
	default:
		return false
	}
}
