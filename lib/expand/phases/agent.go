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

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewAgentStart returns executor that starts an RPC agent on a node
func NewAgentStart(p fsm.ExecutorParams, operator ops.Operator) (*agentStartExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldInstallPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
	}
	proxyClient, err := clients.TeleportProxy(operator, p.Phase.Data.Server.AdvertiseIP)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &agentStartExecutor{
		FieldLogger:    logger,
		TeleportProxy:  proxyClient,
		Master:         *p.Phase.Data.Server,
		Operator:       operator,
		ExecutorParams: p,
	}, nil
}

type agentStartExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// TeleportProxy is teleport proxy client
	TeleportProxy *client.ProxyClient
	// Master is the master node where the agent is deployed
	Master storage.Server
	// Operator is the cluster operator service
	Operator ops.Operator
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute starts an RPC agent on a node
func (p *agentStartExecutor) Execute(ctx context.Context) error {
	deployServer, err := rpc.NewDeployServer(ctx, p.Master, p.TeleportProxy)
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := p.Operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	gravityPackage, err := cluster.App.Manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	err = rpc.DeployAgents(ctx, rpc.DeployAgentsRequest{
		Servers:        []rpc.DeployServer{*deployServer},
		ClusterState:   cluster.ClusterState,
		GravityPackage: *gravityPackage,
		SecretsPackage: loc.RPCSecrets,
		Proxy:          p.TeleportProxy,
		FieldLogger:    p.FieldLogger,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Deployed agent on master node %v.", p.Master.AdvertiseIP)
	return nil
}

// Rollback is no-op for this phase
func (*agentStartExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*agentStartExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*agentStartExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// NewAgentStop returns executor that stops an RPC agent on a node
func NewAgentStop(p fsm.ExecutorParams, operator ops.Operator, packages pack.PackageService) (*agentStopExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldInstallPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
	}
	credentials, err := rpc.ClientCredentialsFromPackage(packages, loc.RPCSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &agentStopExecutor{
		FieldLogger:    logger,
		AgentClient:    fsm.NewAgentRunner(credentials),
		Master:         *p.Phase.Data.Server,
		ExecutorParams: p,
	}, nil
}

type agentStopExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// AgentClient is the RPC agent client
	AgentClient fsm.AgentRepository
	// Master is the master node where the agent is deployed
	Master storage.Server
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute starts an RPC agent on a node
func (p *agentStopExecutor) Execute(ctx context.Context) error {
	err := rpc.ShutdownAgents(ctx, []string{p.Master.AdvertiseIP},
		p.FieldLogger, p.AgentClient)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Stopped agent on master node %v.", p.Master.AdvertiseIP)
	return nil
}

// Rollback is no-op for this phase
func (*agentStopExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*agentStopExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*agentStopExecutor) PostCheck(ctx context.Context) error {
	return nil
}
