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
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewElect returns executor that turns on leader election on the joined node
func NewElect(p fsm.ExecutorParams, operator ops.Operator) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldInstallPhase: p.Phase.ID,
			constants.FieldAdvertiseIP:  p.Phase.Data.Server.AdvertiseIP,
			constants.FieldHostname:     p.Phase.Data.Server.Hostname,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &electExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
	}, nil
}

type electExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the system phase
func (p *electExecutor) Execute(ctx context.Context) error {
	if p.Phase.Data.Server.ClusterRole != string(schema.ServiceRoleMaster) {
		return nil
	}
	p.Progress.NextStep("Enabling leader elections")
	// TODO use etcd client?
	out, err := utils.RunPlanetCommand(ctx, p.FieldLogger, "leader", "resume",
		fmt.Sprintf("--public-ip=%v", p.Phase.Data.Server.AdvertiseIP),
		fmt.Sprintf("--election-key=/planet/cluster/%v/election", p.Plan.ClusterName),
		fmt.Sprintf("--etcd-cafile=%v", defaults.Secret(defaults.RootCertFilename)),
		fmt.Sprintf("--etcd-certfile=%v", defaults.Secret(defaults.EtcdCertFilename)),
		fmt.Sprintf("--etcd-keyfile=%v", defaults.Secret(defaults.EtcdKeyFilename)))
	if err != nil {
		return trace.Wrap(err, "failed to enable leader election: %s", out)
	}
	p.Info("Enabled leader election.")
	return nil
}

// Rollback is no-op for this phase
func (*electExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*electExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*electExecutor) PostCheck(ctx context.Context) error {
	return nil
}
