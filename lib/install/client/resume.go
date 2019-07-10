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
	"github.com/gravitational/gravity/lib/system/environ"
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
	ctx, cancel := context.WithTimeout(ctx, r.ConnectTimeout)
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
		stateDir, err := state.GravityInstallDir()
		if err != nil {
			return trace.Wrap(err)
		}
		r.ServicePath, err = environ.GetServicePath(stateDir)
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
	// installer service connection. Defaults to defaults.ServiceConnectTimeout
	// if unspecified
	ConnectTimeout time.Duration
}
