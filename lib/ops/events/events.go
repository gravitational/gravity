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
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "events")

// Emit saves the provided event to the audit log of the local cluster of the
// provided operator.
func Emit(operator ops.Operator, event string, fields Fields) {
	err := emit(operator, event, fields)
	if err != nil {
		log.Errorf("Failed to emit audit event %v %v: %v.",
			event, fields, trace.DebugReport(err))
	}
}

func emit(operator ops.Operator, event string, fields Fields) error {
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	return operator.EmitAuditEvent(ops.AuditEventRequest{
		SiteKey: cluster.Key(),
		Type:    event,
		Fields:  events.EventFields(fields),
	})
}

const (
	// OperationStarted is emitted when an operation starts.
	OperationStarted = "operation.started"
	// OperationCompleted is emitted when an operation completes successfully.
	OperationCompleted = "operation.completed"
	// OperationFailed is emitted when an operation completes with error.
	OperationFailed = "operation.failed"

	// AppInstalled is emitted when an application image is installed.
	AppInstalled = "application.installed"
	// AppUpgraded is emitted when an application release is upgraded.
	AppUpgraded = "application.upgraded"
	// AppRolledBack is emitted when an application release is rolled back.
	AppRolledBack = "application.rolledback"
	// AppUninstalled is emitted when an application release is uninstalled.
	AppUninstalled = "application.uninstalled"

	// ResourceCreated is emitted when a Gravity resource is created or updated.
	ResourceCreated = "resource.created"
	// ResourceDeleted is emitted when a Gravity resource is deleted.
	ResourceDeleted = "resource.deleted"

	// RemoteSupportEnabled is emitted when cluster enables remote support with an Ops Center.
	RemoteSupportEnabled = "remotesupport.enabled"
	// RemoteSupportDisabled is emitted when cluster disables Ops Center remote support.
	RemoteSupportDisabled = "remotesupport.disabled"

	// UpdatesEnabled is emitted when periodic updates are turned on.
	UpdatesEnabled = "periodicupdates.enabled"
	// UpdatesDisabled is emitted when periodic updates are turned off.
	UpdatesDisabled = "periodicupdates.disabled"
	// UpdatesDownloaded is emitted when periodic updates download an update package.
	UpdatesDownloaded = "periodicupdates.downloaded"

	// ClusterDegraded is emitted when cluster health check fails.
	ClusterDegraded = "cluster.degraded"
	// ClusterActivated is emitted when cluster becomes healthy again.
	ClusterActivated = "cluster.activated"

	// LicenseExpired is emitted when cluster license expires.
	LicenseExpired = "license.expired"
	// LicenseUpdated is emitted when cluster license is updated.
	LicenseUpdated = "license.updated"
	// LicenseGenerated is emitted when a license is generated on Ops Center.
	LicenseGenerated = "license.generated"
)
