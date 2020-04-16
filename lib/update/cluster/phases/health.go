/*
Copyright 2020 Gravitational, Inc.

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

package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/defaults"
	libstatus "github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

type checkLocalHealth struct {
	log.FieldLogger
}

// NewPhaseNodeHealth creates an upgrade phase to check whether the node is healthy
func NewPhaseNodeHealth(logger log.FieldLogger) (*checkLocalHealth, error) {
	return &checkLocalHealth{
		FieldLogger: logger,
	}, nil
}

// Execute will block progress until the node enters a healthy state
func (p *checkLocalHealth) Execute(ctx context.Context) error {
	return trace.Wrap(waitNodeStatus(ctx))
}

// Rollback is no-op for this phase
func (p *checkLocalHealth) Rollback(context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (p *checkLocalHealth) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (p *checkLocalHealth) PostCheck(context.Context) error {
	return nil
}

func waitNodeStatus(ctx context.Context) (err error) {
	b := utils.NewExponentialBackOff(defaults.NodeStatusTimeout)
	err = utils.RetryWithInterval(ctx, b, func() error {
		return trace.Wrap(getLocalNodeStatus(ctx))
	})
	return trace.Wrap(err)
}

func getLocalNodeStatus(ctx context.Context) (err error) {
	var status *libstatus.Agent
	b := utils.NewExponentialBackOff(defaults.NodeStatusTimeout)
	err = utils.RetryTransient(ctx, b, func() error {
		status, err = libstatus.FromLocalPlanetAgent(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if status.GetSystemStatus() != agentpb.SystemStatus_Running {
		return trace.BadParameter("node is degraded")
	}
	return nil
}
