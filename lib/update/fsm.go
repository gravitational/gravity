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

package update

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// FSMConfig is the fsm configuration
type FSMConfig struct {
	// Backend is the cluster etcd backend
	Backend storage.Backend
	// LocalBackend is used as a redundant local storage for cases when etcd goes down
	LocalBackend storage.Backend
	// HostLocalBackend is the host-local backend that stores bootstrap configuration
	// like DNS, logins etc.
	HostLocalBackend storage.Backend
	// Packages is the local package service
	Packages pack.PackageService
	// ClusterPackages is the package service that talks to cluster API
	ClusterPackages pack.PackageService
	// HostLocalPackages is the host-local package service that contains package
	// metadata used for updates
	HostLocalPackages pack.PackageService
	// Apps is the cluster apps service
	Apps app.Applications
	// Client is the cluster Kubernetes client
	Client *kubernetes.Clientset
	// Operator is the local cluster operator
	Operator ops.Operator
	// Users is the cluster identity service
	Users users.Identity
	// Spec is used to retrieve a phase executor, allows
	// plugging different phase executors during tests
	Spec fsm.FSMSpecFunc
	// Remote allows to create RPC clients
	Remote fsm.AgentRepository
}

// NewFSM returns a new FSM instance
func NewFSM(ctx context.Context, c FSMConfig) (*fsm.FSM, error) {
	err := c.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := logrus.WithFields(logrus.Fields{
		trace.Component: "fsm:update",
	})

	updateEngine, err := NewUpdateEngine(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = updateEngine.loadPlan()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = updateEngine.reconcilePlan(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fsm, err := fsm.New(fsm.Config{
		Engine: updateEngine,
		Logger: logger,
		Runner: c.Remote,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return fsm, nil
}

// checkAndSetDefaults validates FSM config and sets defaults
func (c *FSMConfig) checkAndSetDefaults() error {
	if c.Backend == nil {
		return trace.BadParameter("parameter Backend must be set")
	}
	if c.Spec == nil {
		c.Spec = fsmSpec(*c)
	}
	if c.Remote == nil {
		creds, err := fsm.GetClientCredentials()
		if err != nil {
			return trace.Wrap(err)
		}
		c.Remote = fsm.NewAgentRunner(creds)
	}

	return nil
}

// ExecutePhase executes the specified phase
func ExecutePhase(ctx context.Context, config FSMConfig, params fsm.Params, skipVersionCheck bool) error {
	machine, err := NewFSM(ctx, config)
	if err != nil {
		return trace.Wrap(err)
	}

	if !skipVersionCheck {
		err = checkBinaryVersion(machine)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if params.PhaseID == fsm.RootPhase {
		return trace.Wrap(resumeUpdate(ctx, machine, params, config.Remote))
	}

	return trace.Wrap(machine.ExecutePhase(ctx, params))
}

// RollbackPhase rolls back the specified phase, allows fallback to
// recovery fsm in case etcd is down
func RollbackPhase(ctx context.Context, config FSMConfig, params fsm.Params, skipVersionCheck bool) error {
	fsm, err := NewFSM(ctx, config)
	if err != nil {
		return trace.Wrap(err)
	}

	if !skipVersionCheck {
		err = checkBinaryVersion(fsm)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(fsm.RollbackPhase(ctx, params))
}

func resumeUpdate(ctx context.Context, machine *fsm.FSM, p fsm.Params, runner rpc.AgentRepository) error {
	fsmErr := machine.ExecutePlan(ctx, p.Progress, p.Force)
	if fsmErr != nil {
		logrus.Warnf("Failed to execute plan: %v.", fsmErr)
		// fallthrough
	}

	err := machine.Complete(fsmErr)
	if err != nil {
		return trace.Wrap(err)
	}

	if fsmErr != nil {
		return trace.Wrap(fsmErr)
	}

	ctx, cancel := context.WithTimeout(ctx, defaults.RPCAgentShutdownTimeout)
	defer cancel()
	if err = ShutdownClusterAgents(ctx, runner); err != nil {
		logrus.Warnf("Failed to shutdown cluster agents: %v.", trace.DebugReport(err))
	}
	return nil
}
