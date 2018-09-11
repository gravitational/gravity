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
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

func (p *Peer) initOperationPlan(ctx operationContext) error {
	plan, err := ctx.Operator.GetOperationPlan(ctx.Operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if plan != nil {
		return trace.AlreadyExists("plan is already initialized")
	}
	plan, err = p.getOperationPlan(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = ctx.Operator.CreateOperationPlan(ctx.Operation.Key(), *plan)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Info("Initialized operation plan.")
	return nil
}

func (p *Peer) getOperationPlan(ctx operationContext) (*storage.OperationPlan, error) {
	builder, err := p.getPlanBuilder(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan := &storage.OperationPlan{
		OperationID:   ctx.Operation.ID,
		OperationType: ctx.Operation.Type,
		AccountID:     ctx.Operation.AccountID,
		ClusterName:   ctx.Operation.SiteDomain,
		Servers:       builder.ClusterNodes,
	}

	// have cluster controller configure packages for the joining node
	builder.AddConfigurePhase(plan)

	// bootstrap local state on the joining node
	builder.AddBootstrapPhase(plan)

	// download configured packages to the joining node and unpack them
	builder.AddPullPhase(plan)

	// run pre-join hook if the application has it
	if builder.Application.Manifest.HasHook(schema.HookNodeAdding) {
		builder.AddPreHookPhase(plan)
	}

	// install teleport and planet services on the joining node
	builder.AddSystemPhase(plan)

	// when adding a master node, add it to the existing etcd cluster as a full member
	if builder.JoiningNode.Master() {
		// when adding a second master node, etcd cluster becomes unavailable
		// from the moment the second member is added to the moment the planet
		// on the joining node comes up
		//
		// if the planet fails to start, the cluster will stay unhealthy and a
		// special rollback procedure will be required so we're starting an agent
		// on the first master which will be used for recovery
		if len(builder.ClusterNodes.Masters()) == 1 {
			builder.AddStartAgentPhase(plan)
		}
		builder.AddEtcdPhase(plan)
	}

	// wait for the planet to start up and the new Kubernetes node to register
	builder.AddWaitPhase(plan)

	// everything has started correctly so if we started a recovery agent
	// above, we don't need it anymore
	if builder.JoiningNode.Master() && len(builder.ClusterNodes.Masters()) == 1 {
		builder.AddStopAgentPhase(plan)
	}

	// apply labels and taints to the new Kubernetes node
	builder.AddLabelPhase(plan)

	// run post-join hook if the application has it
	if builder.Application.Manifest.HasHook(schema.HookNodeAdded) {
		builder.AddPostHookPhase(plan)
	}

	// if added a master node, make sure it participates in leader election
	if builder.JoiningNode.Master() {
		builder.AddElectPhase(plan)
	}

	return plan, nil
}
