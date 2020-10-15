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
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/system/service"
	"github.com/gravitational/gravity/lib/systemservice"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
	client, err := installpb.NewClient(ctx, installpb.ClientConfig{
		FieldLogger:            r.FieldLogger,
		SocketPath:             r.SocketPath,
		ShouldReconnectService: shouldReconnectService(r.ServiceName),
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to connect to the installer service.\n"+
			"Use 'journalctl -u %v' to check the service logs for errors.\n"+
			"Use 'gravity install' or 'gravity join' to start the installation.", r.ServiceName)
	}
	return client, nil
}

func (r *ResumeStrategy) checkAndSetDefaults() (err error) {
	if r.ServicePath == "" {
		return trace.BadParameter("ServicePath is required")
	}
	if r.ServiceName == "" {
		return trace.BadParameter("ServiceName is required")
	}
	if r.SocketPath == "" {
		return trace.BadParameter("SocketPath is required")
	}
	if r.ConnectTimeout == 0 {
		r.ConnectTimeout = defaults.ServiceConnectTimeout
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:installer")
	}
	return nil
}

func (r *ResumeStrategy) serviceName() string {
	return r.ServiceName
}

// restartService starts the installer's systemd unit unless it's already active
func (r *ResumeStrategy) restartService() error {
	err := service.Start(r.ServiceName)
	if err == nil {
		return nil
	}
	if systemservice.IsUnknownServiceError(err) {
		return trace.NotFound("service %v not found", r.ServiceName)
	}
	return trace.Wrap(err)
}

// ResumeStrategy implements the strategy to connect to the existing installer service
type ResumeStrategy struct {
	// FieldLogger specifies the logger
	log.FieldLogger
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ServicePath specifies the absolute path to the service unit
	ServicePath string
	// ServiceName specifies the name of the installer service. Must reference
	// the same service specified with ServicePath
	ServiceName string
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection. Defaults to defaults.ServiceConnectTimeout
	// if unspecified
	ConnectTimeout time.Duration
}
