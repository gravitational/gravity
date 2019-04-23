package installer

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/system/service"
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
func New(ctx context.Context, config Config) (*Client, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c := &Client{
		FieldLogger: config.FieldLogger,
		Printer:     config.Printer,
		config:      config,
	}
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
func (r *Client) Run(ctx context.Context) error {
	stream, err := r.client.Execute(ctx, &installpb.ExecuteRequest{})
	if err != nil {
		return trace.Wrap(err)
	}
	err = r.progressLoop(stream)
	r.Shutdown(ctx)
	return trace.Wrap(err)
}

// Shutdown signals the service to stop
func (r *Client) Shutdown(ctx context.Context) error {
	_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
	return trace.Wrap(err)
}

// Uninstall invokes the Uninstall API on the service
func (r *Client) Uninstall(ctx context.Context) error {
	_, err := r.client.Uninstall(ctx, &installpb.UninstallRequest{})
	return trace.Wrap(err)
}

// Completed returns true if the operation has already been completed
func (r *Client) Completed() bool {
	return atomic.LoadInt32((*int32)(&r.completed)) == 1
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

func (r *Client) connectRunning(ctx context.Context) error {
	const connectionTimeout = 10 * time.Second
	if _, err := os.Stat(installpb.SocketPath(r.config.OperationStateDir)); err != nil && os.IsNotExist(err) {
		// Fail fast when the socket file has not been created
		return trace.ConvertSystemError(err)
	}
	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()
	client, err := installpb.NewClient(ctx, r.config.OperationStateDir, r.FieldLogger,
		// Fail fast at first non-temporary error
		grpc.FailOnNonTempDialError(true))
	if err != nil {
		return trace.Wrap(err)
	}
	r.client = client
	r.addTerminationHandler()
	return nil
}

func (r *Client) connectNew(ctx context.Context) error {
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
		uninstallService(r.config.ServiceName)
	}()
	if r.config.ConnectTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.config.ConnectTimeout)
		defer cancel()
	}
	client, err := installpb.NewClient(ctx, r.config.OperationStateDir, r.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}
	r.client = client
	r.addTerminationHandler()
	return nil
}

// installSelfAsService installs a systemd unit using the current process's command line
// and turns on service mode
func (r *Client) installSelfAsService() error {
	req := systemservice.NewServiceRequest{
		ServiceSpec: systemservice.ServiceSpec{
			StartCommand: strings.Join(r.config.Args, " "),
			StartPreCommands: []string{
				removeSocketFileCommand(installpb.SocketPath(r.config.OperationStateDir)),
			},
			// FIXME
			// RestartPreventExitStatus: "",
			// TODO(dmitri): run as euid?
			User:    constants.RootUIDString,
			Restart: "on-failure",
		},
		Name:    r.config.ServiceName,
		NoBlock: true,
	}
	r.WithField("req", fmt.Sprintf("%+v", req)).Info("Install service.")
	return trace.Wrap(service.Reinstall(req))
}

// checkLocalState performs a local environment sanity check to make sure
// that install/join on this node can proceed without issues
func (r *Client) checkLocalState() error {
	// make sure that there are no packages in the local state left from
	// some improperly cleaned up installation
	packages, err := r.config.Packages.GetPackages(defaults.SystemAccountOrg)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(packages) != 0 {
		return trace.BadParameter("detected previous installation state in %v, "+
			"please clean it up using `gravity leave --force` before proceeding "+
			"(see https://gravitational.com/gravity/docs/cluster/#deleting-a-cluster for more details)",
			r.config.StateDir)
	}
	return nil
}

func (r *Client) progressLoop(stream installpb.Agent_ExecuteClient) (err error) {
	for {
		resp, err := stream.Recv()
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
				return nil
			}
			if trace.Unwrap(err) == io.EOF {
				// Stream done
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
		if resp.Complete {
			break
		}
	}
	r.markCompleted()
	return nil
}

func (r *Client) addTerminationHandler() {
	r.config.InterruptHandler.AddStopper(signals.StopperFunc(func(ctx context.Context) error {
		_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
		return trace.Wrap(err)
	}))
}

func (r *Client) markCompleted() {
	atomic.StoreInt32((*int32)(&r.completed), 1)
}

// Client implements the client to the installer service
type Client struct {
	log.FieldLogger
	utils.Printer
	config Config
	client installpb.AgentClient
	// completed indicates whether the operation is complete
	completed int32
}

func uninstallService(name string) error {
	return trace.Wrap(service.Uninstall(name))
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
