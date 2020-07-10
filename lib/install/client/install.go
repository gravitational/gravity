/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package client

import (
	"context"
	"fmt"
	"os"
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
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// connect creates the installer services and returns a client.
// It performs host validation to assert whether the host can run the installer
func (r *InstallerStrategy) connect(ctx context.Context) (installpb.AgentClient, error) {
	r.Info("Creating and connecting to new instance.")
	err := r.Validate()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = r.installSelfAsService()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, r.ConnectTimeout)
	defer cancel()
	client, err := installpb.NewClient(ctx, installpb.ClientConfig{
		FieldLogger:     r.FieldLogger,
		SocketPath:      r.SocketPath,
		IsServiceFailed: isServiceFailed(serviceName(r.ServicePath)),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// installSelfAsService installs a systemd unit using the current process's command line
// and turns on service mode
func (r *InstallerStrategy) installSelfAsService() error {
	if err := os.MkdirAll(filepath.Dir(r.SocketPath), defaults.SharedDirMask); err != nil {
		return trace.ConvertSystemError(err)
	}
	req := systemservice.NewServiceRequest{
		ServiceSpec: systemservice.ServiceSpec{
			StartCommand:             strings.Join(r.Args, " "),
			User:                     constants.RootUIDString,
			SuccessExitStatus:        successExitStatuses,
			RestartPreventExitStatus: noRestartExitStatuses,
			// Enable automatic restart of the service
			Restart:          "always",
			Timeout:          int(time.Duration(defaults.ServiceConnectTimeout).Seconds()),
			WantedBy:         "multi-user.target",
			WorkingDirectory: r.ApplicationDir,
			// Propagate all gravity-related environment variables to the service.
			Environment: utils.GetenvsByPrefix(constants.GravityEnvVarPrefix),
		},
		NoBlock: true,
		Name:    r.ServicePath,
	}
	r.WithField("req", fmt.Sprintf("%+v", req)).Info("Install service.")
	return trace.Wrap(service.Reinstall(req))
}

func (r *InstallerStrategy) checkAndSetDefaults() (err error) {
	if len(r.Args) == 0 {
		return trace.BadParameter("Args is required")
	}
	if r.ApplicationDir == "" {
		return trace.BadParameter("ApplicationDir is required")
	}
	if r.Validate == nil {
		return trace.BadParameter("Validate is required")
	}
	if r.ServicePath == "" {
		r.ServicePath, err = state.GravityInstallDir(defaults.GravityRPCInstallerServiceName)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if r.SocketPath == "" {
		r.SocketPath, err = installpb.SocketPath()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if r.ConnectTimeout == 0 {
		r.ConnectTimeout = defaults.ServiceConnectTimeout
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:installer")
	}
	return nil
}

// InstallerStrategy implements the strategy that creates a new installer service
// before attempting to connect.
// This strategy also validates the environment before attempting to set up the service
// to prevent from running the installer on a system already part of the cluster
type InstallerStrategy struct {
	// FieldLogger specifies the logger
	log.FieldLogger
	// Args specifies the service command line including the executable
	Args []string
	// ApplicationDir specifies the directory with installer files
	ApplicationDir string
	// Validate specifies the environment validation function.
	// The service will only be installed when Validate returns nil
	Validate func() error
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ServicePath specifies the absolute path to the service unit
	ServicePath string
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection.
	ConnectTimeout time.Duration
}

// isServiceFailed returns an error if the service has failed.
func isServiceFailed(serviceName string) func() error {
	return func() error {
		failed, err := service.IsFailed(serviceName)
		if err == nil && failed {
			return trace.Errorf("service %q has failed. Check journal log for details.",
				serviceName)
		}
		return trace.Wrap(err)
	}
}

func serviceName(path string) (name string) {
	return filepath.Base(path)
}

var (
	// successExitStatuses lists exit statuses considered a successful exit for the service
	successExitStatuses = strings.Join([]string{
		strconv.Itoa(defaults.AbortedOperationExitCode),
		strconv.Itoa(defaults.CompletedOperationExitCode),
	}, " ")
	// noRestartExitStatuses lists exit statuses that prevent service from getting automatically
	// restarted by systemd
	noRestartExitStatuses = strings.Join([]string{
		strconv.Itoa(defaults.AbortedOperationExitCode),
		strconv.Itoa(defaults.CompletedOperationExitCode),
		strconv.Itoa(defaults.FailedPreconditionExitCode),
	}, " ")
)
