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

package opsservice

import (
	"context"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ValidateServers executes preflight checks and returns formatted error if
// any of the checks fail.
func ValidateServers(ctx context.Context, operator ops.Operator, req ops.ValidateServersRequest) error {
	resp, err := operator.ValidateServers(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	var failed []*agentpb.Probe
	for _, probe := range resp.Probes {
		if probe.Severity != agentpb.Probe_Warning {
			failed = append(failed, probe)
		}
	}
	if len(failed) > 0 {
		return trace.BadParameter("The following pre-flight checks failed:\n%v",
			checks.FormatFailedChecks(failed))
	}
	return nil
}

// ValidateServers runs preflight checks before the installation and returns
// failed probes.
func (o *Operator) ValidateServers(ctx context.Context, req ops.ValidateServersRequest) (*ops.ValidateServersResponse, error) {
	log.Infof("Validating servers: %#v.", req)

	op, err := o.GetSiteOperation(req.OperationKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := o.openSite(req.SiteKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	infos, err := cluster.agentService().GetServerInfos(ctx, op.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	probes, err := ops.CheckServers(ctx, op.Key(), infos, req.Servers,
		cluster.agentService(), cluster.app.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ops.ValidateServersResponse{
		Probes: probes,
	}, nil
}
