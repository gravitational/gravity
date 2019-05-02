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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	installpb "github.com/gravitational/gravity/lib/install/proto"
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
// The client installs the installer service and starts the
// installer operation.
// If restarted, the client will first attempt to connect to a running
// installer service before attempting to set up a new one.
// If no installer service is running, the client will validate that it is
// safe to set up and execute the installer (i.e. validate that the node
// is not already part of the cluster).
//
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
	if config.Resume {
		c.Info("Connecting to running instance.")
		err = c.connectRunning(ctx)
		if err == nil {
			return c, nil
		}
		return nil, trace.Wrap(err, "failed to connect to the installer service.\n"+
			"Use 'gravity install' to start the installation.")
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

// Abort signals that the server clean up the state and shut down
func (r *Client) Abort(ctx context.Context) error {
	_, err := r.client.Abort(ctx, &installpb.AbortRequest{})
	r.Shutdown(ctx)
	return trace.Wrap(err)
}

// Completed returns true if the operation has already been completed
func (r *Client) Completed() bool {
	return atomic.LoadInt32((*int32)(&r.completed)) == 1
}

func (r *Config) checkAndSetDefaults() error {
	if !r.Resume {
		if len(r.Args) == 0 {
			return trace.BadParameter("Args is required")
		}
		if r.StateChecker == nil {
			return trace.BadParameter("StateChecker is required")
		}
	}
	if r.InterruptHandler == nil {
		return trace.BadParameter("InterruptHandler is required")
	}
	if r.Token == "" {
		return trace.BadParameter("Token is required")
	}
	if r.Printer == nil {
		r.Printer = utils.DiscardPrinter
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:installer")
	}
	if r.ServiceName == "" {
		r.ServiceName = defaults.GravityRPCInstallerServiceName
	}
	if r.SocketPath == "" {
		r.SocketPath = installpb.SocketPath(defaults.GravityEphemeralDir)
	}
	if r.ConnectTimeout == 0 {
		r.ConnectTimeout = 10 * time.Minute
	}
	return nil
}

type Config struct {
	log.FieldLogger
	utils.Printer
	*signals.InterruptHandler
	// Args specifies the service command line including the executable
	Args []string
	// StateChecker specifies the local state checker function.
	// The function is only required when not resuming the service
	StateChecker func() error
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection. Wait forever, if unspecified
	ConnectTimeout time.Duration
	// ServiceName specifies the name of the service unit
	ServiceName string
	// Resume specifies whether the existing service should be resumed
	Resume bool
	// Token specifies the validation token
	Token string
}

func (r *Client) connectRunning(ctx context.Context) error {
	r.Info("Restart service.")
	if err := r.restartService(); err != nil {
		return trace.Wrap(err)
	}
	r.Info("Connect to running service.")
	const connectionTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()
	client, err := installpb.NewClient(ctx, r.config.SocketPath, r.FieldLogger,
		// Fail fast at first non-temporary error
		grpc.FailOnNonTempDialError(true))
	if err != nil {
		return trace.Wrap(err)
	}
	r.client = client
	r.addTerminationHandler()
	_, err = client.Handshake(ctx, &installpb.HandshakeRequest{Token: r.config.Token})
	if err != nil {
		if code := status.Code(err); code == codes.PermissionDenied {
			return trace.AccessDenied("wrong service modality.\n" +
				"Are you running 'gravity plan resume' from a join node? " +
				"Try 'gravity join resume' instead.")
		}
		return trace.Wrap(err)
	}
	return nil
}

func (r *Client) connectNew(ctx context.Context) error {
	err := r.config.StateChecker()
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
		r.uninstallService()
	}()
	if r.config.ConnectTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.config.ConnectTimeout)
		defer cancel()
	}
	client, err := installpb.NewClient(ctx, r.config.SocketPath, r.FieldLogger)
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
				removeSocketFileCommand(r.config.SocketPath),
			},
			// TODO(dmitri): run as euid?
			User: constants.RootUIDString,
			// Enable automatic restart of the service
			Restart:  "always",
			WantedBy: "multi-user.target",
		},
		NoBlock: true,
		Unmask:  true,
		// Create service in alternative location to be able to mask
		UnitPath: filepath.Join("/usr/lib/systemd/system", r.config.ServiceName),
	}
	r.WithField("req", fmt.Sprintf("%+v", req)).Info("Install service.")
	return trace.Wrap(service.Reinstall(req))
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
			// Exit upon first error
			return trace.BadParameter(resp.Errors[0].Message)
		}
		r.PrintStep(resp.Message)
		if resp.Complete {
			break
		}
	}
	r.markCompleted()
	r.maskService()
	return nil
}

func (r *Client) addTerminationHandler() {
	r.config.InterruptHandler.AddStopper(signals.AborterFunc(func(ctx context.Context, interrupted bool) (err error) {
		if interrupted {
			_, err = r.client.Abort(ctx, &installpb.AbortRequest{})
		} else {
			_, err = r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
		}
		return trace.Wrap(err)
	}))
}

func (r *Client) markCompleted() {
	atomic.StoreInt32((*int32)(&r.completed), 1)
}

// restartService starts the installer's systemd unit unless it's already active
func (r *Client) restartService() error {
	return trace.Wrap(service.Start(r.config.ServiceName))
}

func (r *Client) uninstallService() error {
	return trace.Wrap(service.Uninstall(systemservice.UninstallServiceRequest{
		Name:       r.config.ServiceName,
		RemoveFile: true,
	}))
}

func (r *Client) maskService() error {
	return trace.Wrap(service.Disable(systemservice.DisableServiceRequest{
		Name: r.config.ServiceName,
		Mask: true,
	}))
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
