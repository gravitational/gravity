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

package expand

import (
	"strings"

	"github.com/gravitational/gravity/lib/expand/phases"
	"github.com/gravitational/gravity/lib/fsm"
	installphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/trace"
)

// FSMSpec returns a function that returns an appropriate phase executor
func FSMSpec(config FSMConfig) fsm.FSMSpecFunc {
	return func(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
		switch {
		case strings.HasPrefix(p.Phase.ID, installphases.ConfigurePhase):
			return installphases.NewConfigure(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, installphases.BootstrapPhase):
			return installphases.NewBootstrap(p,
				config.Operator,
				config.Apps,
				config.LocalBackend,
				remote)

		case strings.HasPrefix(p.Phase.ID, installphases.PullPhase):
			return installphases.NewPull(p,
				config.Operator,
				config.Packages,
				config.LocalPackages,
				config.Apps,
				config.LocalApps,
				remote)

		case strings.HasPrefix(p.Phase.ID, PreHookPhase):
			return installphases.NewHook(p,
				config.Operator,
				config.LocalApps,
				schema.HookNodeAdding)

		case strings.HasPrefix(p.Phase.ID, EtcdPhase):
			return phases.NewEtcd(p,
				config.Operator,
				config.Etcd)

		case strings.HasPrefix(p.Phase.ID, SystemPhase):
			return installphases.NewSystem(p,
				config.Operator,
				remote)

		case strings.HasPrefix(p.Phase.ID, PostHookPhase):
			return installphases.NewHook(p,
				config.Operator,
				config.LocalApps,
				schema.HookNodeAdded)

		default:
			return nil, trace.BadParameter("unknown phase %q", p.Phase.ID)
		}
	}
}

const (
	// PreHookPhase runs pre-expand application hook
	PreHookPhase = "/pre"
	// EtcdPhase adds joining node to cluster's etcd cluster
	EtcdPhase = "/etcd"
	// SystemPhase installs system software on the joining node
	SystemPhase = "/system"
	// PostHookPhase runs post-expand application hook
	PostHookPhase = "/post"
)
