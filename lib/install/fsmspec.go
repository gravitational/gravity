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

package install

import (
	"strings"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/install/phases"

	"github.com/gravitational/trace"
)

// FSMSpec returns a function that returns an appropriate phase executor
// based on the provided params
func FSMSpec(config FSMConfig) fsm.FSMSpecFunc {
	return func(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
		switch {
		case p.Phase.ID == phases.ChecksPhase:
			return phases.NewChecks(p,
				config.Operator,
				config.OperationKey)

		case p.Phase.ID == phases.ConfigurePhase:
			return phases.NewConfigure(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, phases.BootstrapPhase):
			return phases.NewBootstrap(p,
				config.Operator,
				config.Apps,
				config.LocalBackend, remote)

		case strings.HasPrefix(p.Phase.ID, phases.PullPhase):
			return phases.NewPull(p,
				config.Operator,
				config.Packages,
				config.LocalPackages,
				config.Apps,
				config.LocalApps, remote)

		case strings.HasPrefix(p.Phase.ID, phases.MastersPhase), strings.HasPrefix(p.Phase.ID, phases.NodesPhase):
			return phases.NewSystem(p,
				config.Operator, remote)

		case p.Phase.ID == phases.WaitPhase:
			return phases.NewWait(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, phases.LabelPhase):
			client, err := httplib.GetUnprivilegedKubeClient(config.DNSConfig.Addr())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return phases.NewNodes(p,
				config.Operator,
				config.LocalApps,
				client)

		case p.Phase.ID == phases.RBACPhase:
			client, err := httplib.GetClusterKubeClient(config.DNSConfig.Addr())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return phases.NewRBAC(p,
				config.Operator,
				config.LocalApps,
				client)
		case p.Phase.ID == phases.CorednsPhase:
			client, err := httplib.GetClusterKubeClient(config.DNSConfig.Addr())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return phases.NewCorednsPhase(p,
				config.Operator,
				client)

		case p.Phase.ID == phases.ResourcesPhase:
			return phases.NewResources(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, phases.ExportPhase):
			return phases.NewExport(p,
				config.Operator,
				config.LocalPackages,
				config.LocalApps, remote)

		case strings.HasPrefix(p.Phase.ID, phases.RuntimePhase), strings.HasPrefix(p.Phase.ID, phases.AppPhase):
			return phases.NewApp(p,
				config.Operator,
				config.LocalApps)

		case strings.HasPrefix(p.Phase.ID, phases.EnableElectionPhase):
			return phases.NewEnableElectionPhase(p, config.Operator)

		default:
			return nil, trace.BadParameter("unknown phase %q", p.Phase.ID)
		}
	}
}
