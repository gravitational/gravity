/*
Copyright 2018-2019 Gravitational, Inc.

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
	"context"
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
		case strings.HasPrefix(p.Phase.ID, installphases.InitPhase):
			return installphases.NewInit(p,
				config.Operator,
				config.Apps,
				config.Packages)

		case strings.HasPrefix(p.Phase.ID, installphases.BootstrapSELinuxPhase):
			return installphases.NewSELinux(p,
				config.Operator,
				config.Apps)

		case strings.HasPrefix(p.Phase.ID, ChecksPhase):
			return phases.NewChecks(p,
				config.Operator,
				config.Runner)

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
				config.Apps,
				schema.HookNodeAdding)

		case strings.HasPrefix(p.Phase.ID, StartAgentPhase):
			return phases.NewAgentStart(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, StopAgentPhase):
			return phases.NewAgentStop(p,
				config.Operator,
				config.Packages)

		case strings.HasPrefix(p.Phase.ID, EtcdBackupPhase):
			return phases.NewEtcdBackup(p,
				config.Operator,
				config.Runner)

		case strings.HasPrefix(p.Phase.ID, EtcdPhase):
			return phases.NewEtcd(p,
				config.Operator,
				config.Runner)

		case strings.HasPrefix(p.Phase.ID, PushAppToRegistry):
			return phases.NewPushAppToRegistry(context.TODO(), p,
				config.Operator,
				config.Apps)

		case strings.HasPrefix(p.Phase.ID, SystemPhase):
			return installphases.NewSystem(p,
				config.Operator,
				config.LocalPackages,
				remote)

		case strings.HasPrefix(p.Phase.ID, WaitPlanetPhase):
			return phases.NewWaitPlanet(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, WaitK8sPhase):
			return phases.NewWaitK8s(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, WaitTeleportPhase):
			return phases.NewWaitTeleport(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, PostHookPhase):
			return installphases.NewHook(p,
				config.Operator,
				config.Apps,
				schema.HookNodeAdded)

		case strings.HasPrefix(p.Phase.ID, ElectPhase):
			return phases.NewElect(p,
				config.Operator)

		default:
			return nil, trace.BadParameter("unknown phase %q", p.Phase.ID)
		}
	}
}

const (
	// ChecksPhase runs preflight checks on the joining node
	ChecksPhase = "/checks"
	// PreHookPhase runs pre-expand application hook
	PreHookPhase = "/preHook"
	// EtcdBackupPhase backs up etcd data on a master node
	EtcdBackupPhase = "/etcdBackup"
	// EtcdPhase adds joining node to cluster's etcd cluster
	EtcdPhase = "/etcd"
	// SystemPhase installs system software on the joining node
	SystemPhase = "/system"
	// WaitPlanetPhase waits for planet to start
	WaitPlanetPhase = "/wait/planet"
	// WaitK8sPhase waits for joining node to register with Kubernetes
	WaitK8sPhase = "/wait/k8s"
	// PushAppToRegistry pushes the current application images to the local
	// registry
	PushAppToRegistry = "/pushToRegistry"
	// WaitTeleportPhase waits for Teleport on the joining node to join the cluster
	WaitTeleportPhase = "/wait/teleport"
	// PostHookPhase runs post-expand application hook
	PostHookPhase = "/postHook"
	// ElectPhase enables leader election on master node
	ElectPhase = "/elect"
	// StartAgentPhase starts RPC agent
	StartAgentPhase = "/startAgent"
	// StopAgentPhase stops RPC agent
	StopAgentPhase = "/stopAgent"
)
