package installer

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/system/service"
	"github.com/gravitational/gravity/lib/systemservice"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// connect creates the installer services and returns a client.
// It performs host validation to assert whether the host can run the installer
func (r *InstallerStrategy) connect(ctx context.Context) (installpb.AgentClient, error) {
	r.Info("Creating and connecting to new instance.")
	err := r.StateChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = r.installSelfAsService()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err == nil {
			return
		}
		r.uninstallService()
	}()
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, r.ConnectTimeout)
	defer cancel()
	client, err := installpb.NewClient(ctx, r.SocketPath, r.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// installSelfAsService installs a systemd unit using the current process's command line
// and turns on service mode
func (r *InstallerStrategy) installSelfAsService() error {
	req := systemservice.NewServiceRequest{
		ServiceSpec: systemservice.ServiceSpec{
			StartCommand: strings.Join(r.Args, " "),
			StartPreCommands: []string{
				removeSocketFileCommand(r.SocketPath),
			},
			// TODO(dmitri): run as euid?
			User:                     constants.RootUIDString,
			RestartPreventExitStatus: strconv.Itoa(defaults.AbortedOperationExitCode),
			// Enable automatic restart of the service
			Restart:  "always",
			WantedBy: "multi-user.target",
		},
		NoBlock: true,
		Unmask:  true,
		Name:    r.ServicePath,
	}
	r.WithField("req", fmt.Sprintf("%+v", req)).Info("Install service.")
	return trace.Wrap(service.Reinstall(req))
}

func (r *InstallerStrategy) uninstallService() error {
	return trace.Wrap(service.Uninstall(systemservice.UninstallServiceRequest{
		Name:       r.serviceName(),
		RemoveFile: true,
	}))
}

func (r *InstallerStrategy) serviceName() (name string) {
	return filepath.Base(r.ServicePath)
}

func (r *InstallerStrategy) checkAndSetDefaults() error {
	if len(r.Args) == 0 {
		return trace.BadParameter("Args is required")
	}
	if r.StateChecker == nil {
		return trace.BadParameter("StateChecker is required")
	}
	if r.ServicePath == "" {
		r.ServicePath = state.GravityInstallDir(defaults.GravityRPCInstallerServiceName)
	}
	if r.SocketPath == "" {
		r.SocketPath = installpb.SocketPath()
	}
	if r.ConnectTimeout == 0 {
		r.ConnectTimeout = 10 * time.Minute
	}
	return nil
}

type InstallerStrategy struct {
	log.FieldLogger
	// Args specifies the service command line including the executable
	Args []string
	// StateChecker specifies the local state checker function.
	StateChecker func() error
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ServicePath specifies the absolute path to the service unit
	ServicePath string
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection. Wait forever, if unspecified
	ConnectTimeout time.Duration
}
