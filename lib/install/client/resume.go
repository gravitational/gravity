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
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/system/service"
	"github.com/gravitational/gravity/lib/utils"

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

func (r *ResumeStrategy) checkAndSetDefaults() (err error) {
	if r.ServicePath == "" {
		r.ServicePath, err = GetServicePath(state.GravityInstallDir())
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if r.SocketPath == "" {
		r.SocketPath = installpb.SocketPath()
	}
	if r.ConnectTimeout == 0 {
		r.ConnectTimeout = 10 * time.Minute
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:installer")
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

// ResumeStrategy implements the strategy to connect to the existing installer service
type ResumeStrategy struct {
	// FieldLogger specifies the logger
	log.FieldLogger
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ServicePath specifies the absolute path to the service unit
	ServicePath string
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection. Wait forever, if unspecified
	ConnectTimeout time.Duration
}

// GetServicePath returns the name of the service configured in the specified state directory stateDir
func GetServicePath(stateDir string) (path string, err error) {
	for _, name := range []string{
		defaults.GravityRPCInstallerServiceName,
		defaults.GravityRPCAgentServiceName,
	} {
		if ok, _ := utils.IsFile(filepath.Join(stateDir, name)); ok {
			return filepath.Join(stateDir, name), nil
		}
	}
	return "", trace.NotFound("no service unit file in %v", stateDir)
}
