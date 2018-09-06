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

/*

PHASES:

/prechecks

/configure -> gravity-site configures packages

/bootstrap -> setup directories/volumes on the new node, devicemapper, log into site

/pull -> pull configured packages on the new node

/pre -> run preExpand hook

/etcd -> add etcd member

/system -> install teleport/planet units

/wait
  /planet -> wait for planet to come up and check etcd cluster health
  /k8s -> wait for new node to register with k8s

/post -> run postExpand hook

/elect -> resume leader election (if master)

*/

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
		Servers:       builder.Nodes,
	}

	builder.AddConfigurePhase(plan)

	builder.AddBootstrapPhase(plan)

	builder.AddPullPhase(plan)

	if builder.Application.Manifest.HasHook(schema.HookNodeAdding) {
		builder.AddPreHookPhase(plan)
	}

	builder.AddSystemPhase(plan)

	builder.AddEtcdPhase(plan)

	builder.AddWaitPhase(plan)

	builder.AddLabelPhase(plan)

	if builder.Application.Manifest.HasHook(schema.HookNodeAdded) {
		builder.AddPostHookPhase(plan)
	}

	if builder.Node.ClusterRole == string(schema.ServiceRoleMaster) {
		builder.AddElectPhase(plan)
	}

	return plan, nil
}
