package installer

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gravitational/gravity/lib/defaults"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// New returns a new client to handle the installer case.
// See docs/design/client/install.puml for details
func New(ctx context.Context, config Config) (*client, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c := &client{Config: config}
	c.WithField("config", fmt.Sprintf("%+v", config)).Info("Starting installer client.")
	c.Info("Connecting to running instance.")
	err = c.connectRunning(ctx)
	if err == nil {
		return c, nil
	}
	c.Info("Creating and connecting to new instance.")
	err = c.connectNew(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// Run starts the service operation and runs the loop to fetch and display
// operation progress
func (r *client) Run(ctx context.Context) error {
	stream, err := r.client.Execute(ctx, &installpb.ExecuteRequest{})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.progressLoop(ctx, stream))
}

func (r *Config) checkAndSetDefaults() error {
	if len(r.Args) == 0 {
		return trace.BadParameter("Args is required")
	}
	if r.StateDir == "" {
		return trace.BadParameter("StateDir is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("Packages is required")
	}
	if r.InterruptHandler == nil {
		return trace.BadParameter("InterruptHandler is required")
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
	*signals.InterruptHandler
	// Args specifies the service command line including the executable
	Args []string
	// StateDir specifies the install state directory on local host
	StateDir string
	// OperationStateDir specifies the ephemeral state directory used during the oepration
	OperationStateDir string
	// Packages specifies the host-local package service
	Packages pack.PackageService
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection. Wait forever, if unspecified
	ConnectTimeout time.Duration
	// ServiceName specifies the name of the service unit
	ServiceName string
}

func (r *client) connectRunning(ctx context.Context) error {
	if _, err := os.Stat(installpb.SocketPath(r.OperationStateDir)); err != nil && os.IsNotExist(err) {
		// Fail fast when the socket file has not been created
		return trace.ConvertSystemError(err)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := installpb.NewClient(ctx, r.OperationStateDir, r.FieldLogger,
		// Fail fast at first non-temporary error
		grpc.FailOnNonTempDialError(true))
	if err != nil {
		return trace.Wrap(err)
	}
	r.client = client
	r.addTerminationHandler()
	return nil
}

func (r *client) connectNew(ctx context.Context) error {
	err := r.checkLocalState()
	if err != nil {
		return trace.Wrap(err)
	}
	err = r.installSelfAsService()
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err == nil {
			return
		}
		uninstallService(r.ServiceName)
	}()
	if r.ConnectTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.ConnectTimeout)
		defer cancel()
	}
	client, err := installpb.NewClient(ctx, r.OperationStateDir, r.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}
	r.client = client
	r.addTerminationHandler()
	return nil
}

// installSelfAsService installs a systemd unit using the current process's command line
// and turns on service mode
func (r *client) installSelfAsService() error {
	uid := strconv.FormatInt(int64(os.Geteuid()), 32)
	req := systemservice.NewServiceRequest{
		ServiceSpec: systemservice.ServiceSpec{
			// Output the gravity binary version once started
			StartCommand: fmt.Sprintf("%v version", utils.Exe.Path),
			StartPreCommands: []string{
				removeSocketFileCommand(installpb.SocketPath(r.OperationStateDir)),
				strings.Join(r.Args, " "),
			},
			User: uid,
			// FIXME: does not work with one-shot
			// Restart:         "on-failure",
			RemainAfterExit: false,
		},
		Name:    r.ServiceName,
		NoBlock: true,
	}
	r.WithField("req", fmt.Sprintf("%+v", req)).Info("Install service.")
	return trace.Wrap(systemservice.ReinstallOneshotService(req))
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

func (r *client) progressLoop(ctx context.Context, stream installpb.Agent_ExecuteClient) error {
	var resp installpb.ProgressResponse
	for !resp.Complete && !isDone(ctx.Done()) {
		resp, err := stream.Recv()
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
				return nil
			}
			if trace.Unwrap(err) == io.EOF {
				return nil
			}
			r.WithError(err).Warn("Failed to fetch progress.")
			return trace.Wrap(err)
		}
		if len(resp.Errors) != 0 {
			r.PrintStep(color.RedString(resp.Errors[0].Message))
			// Break the client on errors
			return trace.BadParameter(resp.Errors[0].Message)
		}
		r.PrintStep(resp.Message)
	}
	return nil
}

func (r *client) addTerminationHandler() {
	r.InterruptHandler.Add(signals.StopperFunc(func(ctx context.Context) error {
		_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
		return trace.Wrap(err)
	}))
}

type client struct {
	Config
	client installpb.AgentClient
}

func uninstallService(name string) error {
	return trace.Wrap(systemservice.UninstallService(name))
}

func isDone(doneC <-chan struct{}) bool {
	select {
	case <-doneC:
		return true
	default:
		return false
	}
}

func removeSocketFileCommand(socketPath string) (cmd string) {
	return fmt.Sprintf("/usr/bin/rm -f %v", socketPath)
}

func userUnitPath(service string, user user.User) (path string, err error) {
	dir := filepath.Join(user.HomeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(dir, defaults.SharedDirMask); err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return filepath.Join(dir, service), nil
}
