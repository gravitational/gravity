// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package phases

import (
	"context"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewConnect returns executor for the "connect" phase that connects
// installed cluster to an Ops Center
func NewConnect(p fsm.ExecutorParams, operator ossops.Operator) (fsm.PhaseExecutor, error) {
	// the cluster should already be up at this point
	clusterOperator, err := environment.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &connectExecutor{
		FieldLogger:     logger,
		ClusterOperator: clusterOperator,
		ExecutorParams:  p,
	}, nil
}

type connectExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ClusterOperator is the ops client for the local gravity cluster
	ClusterOperator ops.Operator
	// ExecutorParams contains executor params
	fsm.ExecutorParams
}

// Execute connects cluster to an Ops Center
func (p *connectExecutor) Execute(ctx context.Context) error {
	trustedCluster, err := storage.UnmarshalTrustedCluster(
		p.Phase.Data.TrustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Progress.NextStep("Connecting to the Gravity Hub %v",
		trustedCluster.GetName())
	err = p.ClusterOperator.UpsertTrustedCluster(ctx, ossops.SiteKey{
		AccountID:  p.Plan.AccountID,
		SiteDomain: p.Plan.ClusterName,
	}, trustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Connected to the Gravity Hub %v.", trustedCluster.GetName())
	return nil
}

// Rollback is no-op for this phase
func (*connectExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure the phase is executed on a master node
func (p *connectExecutor) PreCheck(ctx context.Context) error {
	err := fsm.CheckMasterServer(p.Plan.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*connectExecutor) PostCheck(ctx context.Context) error {
	return nil
}
