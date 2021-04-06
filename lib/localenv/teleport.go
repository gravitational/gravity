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

package localenv

import (
	"context"

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"

	"github.com/gravitational/teleport/lib/client"
	teleevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

// TeleportClient returns a new teleport client for the local cluster
func (env *LocalEnvironment) TeleportClient(ctx context.Context, proxyHost string) (*client.TeleportClient, error) {
	operator, err := env.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err, "failed to get cluster operator service")
	}
	cluster, err := operator.GetLocalSite(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clients.Teleport(operator, proxyHost, cluster.Domain)
}

// AuditLog returns the cluster audit log service
func (env *LocalEnvironment) AuditLog(ctx context.Context) (teleevents.IAuditLog, error) {
	operator, err := env.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := operator.GetLocalSite(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clients.TeleportAuth(ctx, operator, constants.Localhost, cluster.Domain)
}

// EmitAuditEvent saves the specified event in the audit log of the local cluster.
func (env *LocalEnvironment) EmitAuditEvent(ctx context.Context, event teleevents.Event, fields events.Fields) {
	if err := httplib.InGravity(env.DNS.Addr()); err != nil {
		return // Not inside Gravity cluster.
	}
	operator, err := env.SiteOperator()
	if err != nil {
		log.Errorf(trace.DebugReport(err))
	} else {
		events.Emit(ctx, operator, event, fields)
	}
}

// EmitOperationEvent emits audit event for the provided operation.
func (env *LocalEnvironment) EmitOperationEvent(ctx context.Context, operation ops.SiteOperation) error {
	event, err := events.EventForOperation(operation)
	if err != nil {
		return trace.Wrap(err)
	}
	fields := events.FieldsForOperation(operation)
	env.EmitAuditEvent(ctx, event, fields)
	return nil
}
