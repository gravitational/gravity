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

package events

import (
	"context"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "events")

// Emit saves the provided event to the audit log of the local cluster of the
// provided operator.
func Emit(ctx context.Context, operator ops.Operator, event string, fields Fields) {
	err := emit(ctx, operator, event, fields)
	if err != nil {
		log.Errorf("Failed to emit audit event %v %v: %v.",
			event, fields, trace.DebugReport(err))
	}
}

func emit(ctx context.Context, operator ops.Operator, event string, fields Fields) error {
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	if fields[FieldUser] == "" && storage.UserFromContext(ctx) != "" {
		fields[FieldUser] = storage.UserFromContext(ctx)
	}
	return operator.EmitAuditEvent(ctx, ops.AuditEventRequest{
		SiteKey: cluster.Key(),
		Type:    event,
		Fields:  events.EventFields(fields),
	})
}

const (
	// OperationStarted fires when an operation starts.
	OperationStarted = "operation.started"
	// OperationCompleted fires when an operation completes successfully.
	OperationCompleted = "operation.completed"
	// OperationFailed fires when an operation completes with error.
	OperationFailed = "operation.failed"

	// AppInstalled fires when an application image is installed.
	AppInstalled = "application.installed"
	// AppUpgraded fires when an application release is upgraded.
	AppUpgraded = "application.upgraded"
	// AppRolledBack fires when an application release is rolled back.
	AppRolledBack = "application.rolledback"
	// AppUninstalled fires when an application release is uninstalled.
	AppUninstalled = "application.uninstalled"

	// ResourceCreated fires when a Gravity resource is created or updated.
	ResourceCreated = "resource.created"
	// ResourceDeleted fires when a Gravity resource is deleted.
	ResourceDeleted = "resource.deleted"

	// InviteCreated fires when a new user invitation is generated.
	InviteCreated = "invite.created"

	// ClusterDegraded fires when cluster health check fails.
	ClusterDegraded = "cluster.degraded"
	// ClusterActivated fires when cluster becomes healthy again.
	ClusterActivated = "cluster.activated"
)
