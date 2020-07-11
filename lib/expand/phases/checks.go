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
	"github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/fatih/color"
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
	return &checksExecutor{
		FieldLogger:    logger,
		Runner:         runner,
		Operator:       operator,
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
	// ExecutorParams is common executor params.
	fsm.ExecutorParams
}

// Execute executes preflight checks on the joining node.
func (p *checksExecutor) Execute(ctx context.Context) error {
	master, err := checks.GetServer(ctx, p.Runner, *p.Phase.Data.Master)
	if err != nil {
		return trace.Wrap(err)
	}
	node, err := checks.GetServer(ctx, p.Runner, *p.Phase.Data.Server)
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := p.Operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	reqs, err := checks.RequirementsFromManifest(cluster.App.Manifest)
	if err != nil {
		return trace.Wrap(err)
	}
	checker, err := checks.New(checks.Config{
		Remote:       checks.NewRemote(p.Runner),
		Servers:      []checks.Server{*master, *node},
		Manifest:     cluster.App.Manifest,
		Requirements: reqs,
		Features: checks.Features{
			TestEtcdDisk: true,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// For multi-node checks, use one of master nodes as an "anchor" so
	// the joining node will be compared against that master (e.g. for
	// the OS check, time drift check, etc).
	probes := checker.CheckNode(ctx, *node)
	probes = append(probes, checker.CheckNodes(ctx, []checks.Server{*master, *node})...)
	// Sort probes out into warnings and real failures.
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

// Rollback is no-op for this phase.
func (*checksExecutor) Rollback(context.Context) error { return nil }

// PreCheck is no-op for this phase.
func (*checksExecutor) PreCheck(context.Context) error { return nil }

// PostCheck is no-op for this phase.
func (*checksExecutor) PostCheck(context.Context) error { return nil }
