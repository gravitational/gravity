package installer

import (
	"context"
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/system/service"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// connect connects to the running installer service and returns a client
func (r *ResumeStrategy) connect(ctx context.Context) (installpb.AgentClient, error) {
	r.Info("Restart service.")
	if err := r.restartService(); err != nil {
		return nil, trace.Wrap(err)
	}
	r.Info("Connect to running service.")
	const connectionTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()
	client, err := installpb.NewClient(ctx, r.SocketPath, r.FieldLogger,
		// Fail fast at first non-temporary error
		grpc.FailOnNonTempDialError(true))
	if err != nil {
		return nil, trace.Wrap(err, "failed to connect to the installer service.\n"+
			"Use 'gravity install' to start the installation.")
	}
	return client, nil
}

func (r *ResumeStrategy) checkAndSetDefaults() error {
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

// restartService starts the installer's systemd unit unless it's already active
func (r *ResumeStrategy) restartService() error {
	return trace.Wrap(service.Start(r.serviceName()))
}

func (r *ResumeStrategy) serviceName() (name string) {
	return filepath.Base(r.ServicePath)
}

type ResumeStrategy struct {
	log.FieldLogger
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ServicePath specifies the absolute path to the service unit
	ServicePath string
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection. Wait forever, if unspecified
	ConnectTimeout time.Duration
}
