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

package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"

	"github.com/fatih/color"
	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewChecks returns executor that executes preflight checks on the joining node.
func NewChecks(p fsm.ExecutorParams, operator ops.Operator, runner rpc.AgentRepository) (*checksExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         opKey(p.Plan),
		Operator:    operator,
	}
	cluster, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqs, err := checks.RequirementsFromManifest(cluster.App.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &checksExecutor{
		FieldLogger:    logger,
		Runner:         runner,
		Operator:       operator,
		Cluster:        cluster,
		Requirements:   reqs,
		ExecutorParams: p,
	}, nil
}

type checksExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// Runner is used to run remote commands.
	Runner rpc.AgentRepository
	// Operator is the cluster operator service.
	Operator ops.Operator
	// Cluster is the local cluster.
	Cluster *ops.Site
	// Requirements is the validation requirements from cluster manifest.
	Requirements map[string]checks.Requirements
	// ExecutorParams is common executor params.
	fsm.ExecutorParams
}

// Execute executes preflight checks on the joining node.
func (p *checksExecutor) Execute(ctx context.Context) (err error) {
	var checker checks.Checker
	if p.Phase.Data.Server.IsMaster() {
		checker, err = p.getMasterChecker(ctx)
	} else {
		checker, err = p.getNodeChecker(ctx)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	probes := checker.Check(ctx)
	// Sort probes out into warnings and hard failures.
	var failed []*agentpb.Probe
	for _, probe := range probes {
		if probe.Status != agentpb.Probe_Failed {
			continue
		}
		if probe.Severity == agentpb.Probe_Warning {
			p.Progress.NextStep(color.YellowString(probe.Detail))
		}
		if probe.Severity == agentpb.Probe_Critical {
			p.Progress.NextStep(color.RedString(probe.Detail))
			failed = append(failed, probe)
		}
	}
	if len(failed) != 0 {
		return trace.BadParameter("The following checks failed:\n%v",
			checks.FormatFailedChecks(failed))
	}
	return nil
}

// getMasterChecker returns a checker that performs checks when adding a master node.
//
// In addition to the local node checks, it also makes sure that there's no
// time drift between this and other master which is important since it's
// going to run a full etcd member.
func (p *checksExecutor) getMasterChecker(ctx context.Context) (checks.Checker, error) {
	master, err := checks.GetServer(ctx, p.Runner, *p.Phase.Data.Master)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	node, err := checks.GetServer(ctx, p.Runner, *p.Phase.Data.Server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return checks.New(checks.Config{
		Remote:       checks.NewRemote(p.Runner),
		Servers:      []checks.Server{*master, *node},
		Manifest:     p.Cluster.App.Manifest,
		Requirements: p.Requirements,
		Features: checks.Features{
			TestEtcdDisk: true,
		},
	})
}

// getNodeChecker returns a checker that performs checks when adding a regular node.
func (p *checksExecutor) getNodeChecker(ctx context.Context) (checks.Checker, error) {
	node, err := checks.GetServer(ctx, p.Runner, *p.Phase.Data.Server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return checks.New(checks.Config{
		Remote:       checks.NewRemote(p.Runner),
		Servers:      []checks.Server{*node},
		Manifest:     p.Cluster.App.Manifest,
		Requirements: p.Requirements,
		Features: checks.Features{
			TestEtcdDisk: true,
		},
	})
}

// Rollback is no-op for this phase.
func (*checksExecutor) Rollback(context.Context) error { return nil }

// PreCheck is no-op for this phase.
func (*checksExecutor) PreCheck(context.Context) error { return nil }

// PostCheck is no-op for this phase.
func (*checksExecutor) PostCheck(context.Context) error { return nil }
