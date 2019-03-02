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
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/teleport/lib/events"
)

// Fields defines event fields.
//
// It is an alias for Teleport's event fields so callers who emit events
// do not have to import two packages.
type Fields events.EventFields

// FieldsForOperation returns event fields for the provided operation.
func FieldsForOperation(operation ops.SiteOperation) Fields {
	fields := Fields{
		FieldOperationID:   operation.ID,
		FieldOperationType: operation.Type,
	}
	switch operation.Type {
	case ops.OperationExpand:
		servers := operation.Servers
		if len(servers) > 0 {
			fields[FieldNodeIP] = servers[0].AdvertiseIP
			fields[FieldNodeHostname] = servers[0].Hostname
			fields[FieldNodeRole] = servers[0].Role
		}
	case ops.OperationShrink:
		servers := operation.Shrink.Servers
		if len(servers) > 0 {
			fields[FieldNodeIP] = servers[0].AdvertiseIP
			fields[FieldNodeHostname] = servers[0].Hostname
			fields[FieldNodeRole] = servers[0].Role
		}
	case ops.OperationUpdate:
		fields[FieldUpdate] = operation.Update.UpdatePackage
	}
	return fields
}

// FieldsForRelease returns event fields for the provided application release.
func FieldsForRelease(release helm.Release) Fields {
	return Fields{
		FieldName:        release.ChartName,
		FieldVersion:     release.ChartVersion,
		FieldReleaseName: release.Name,
	}
}

const (
	// FieldOperationID contains ID of the operation.
	FieldOperationID = "id"
	// FieldOperationType contains type of the operation.
	FieldOperationType = "type"
	// FieldNodeIP contains IP of the joining/leaving node.
	FieldNodeIP = "ip"
	// FieldNodeHostname contains hostname of the joining/leaving node.
	FieldNodeHostname = "hostname"
	// FieldNodeRole contains role of the joining/leaving node.
	FieldNodeRole = "role"
	// FieldUpdate contains the update package.
	FieldUpdate = "update"
	// FieldName contains name, e.g. resource name, application name, etc.
	FieldName = "name"
	// FieldKind contains resource kind.
	FieldKind = "kind"
	// FieldUser contains resource user.
	FieldUser = "user"
	// FieldReleaseName contains application release name.
	FieldReleaseName = "releaseName"
	// FieldPackage contains application package name.
	FieldPackage = "package"
	// FieldVersion contains application package version.
	FieldVersion = "version"
	// FieldInterval contains time interval, e.g. for periodic updates.
	FieldInterval = "interval"
	// FieldReason contains cluster deactivation reason.
	FieldReason = "reason"
	// FieldTime contains event time.
	FieldTime = "time"
	// FieldExpires contains expiration time, e.g. for license.
	FieldExpires = "expires"
)
