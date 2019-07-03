/*
Copyright 2018 Gravitational, Inc.

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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewEnableElectionPhase returns an executor to enable election for all master nodes in the cluster
func NewEnableElectionPhase(p fsm.ExecutorParams, operator ops.Operator) (fsm.PhaseExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.Server == nil {
		return nil, trace.BadParameter("server is required")
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase:       p.Phase.ID,
			constants.FieldAdvertiseIP: p.Phase.Data.Server.AdvertiseIP,
			constants.FieldHostname:    p.Phase.Data.Server.Hostname,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &enableElectionExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
	}, nil
}

type enableElectionExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the system phase
func (p *enableElectionExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Enable leader elections")
	p.Info("Enable leader elections")

	for _, server := range p.Plan.Servers {
		if server.ClusterRole == string(schema.ServiceRoleMaster) {
			err := ops.EnableLeaderElection(ctx, p.Plan.ClusterName, server, p.FieldLogger)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// Rollback is no-op for this phase
func (*enableElectionExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (p *enableElectionExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*enableElectionExecutor) PostCheck(ctx context.Context) error {
	return nil
}
