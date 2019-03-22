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
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

// Fields defines event fields.
//
// It is an alias for Teleport's event fields so callers who emit events
// do not have to import two packages.
type Fields events.EventFields

// WithField returns a copy of these fields with an additional provided field.
func (f Fields) WithField(field string, value interface{}) Fields {
	copy := make(map[string]interface{})
	for k, v := range f {
		copy[k] = v
	}
	copy[field] = value
	return Fields(copy)
}

// FieldsForOperation returns event fields for the provided operation.
func FieldsForOperation(operation ops.SiteOperation) Fields {
	fields, err := fieldsForOperation(operation)
	if err != nil {
		log.Errorf(trace.DebugReport(err))
	}
	return fields
}

func fieldsForOperation(operation ops.SiteOperation) (Fields, error) {
	fields := Fields{
		FieldOperationID:   operation.ID,
		FieldOperationType: operation.Type,
		FieldCluster:       operation.SiteDomain,
		FieldUser:          operation.CreatedBy,
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
		if operation.Shrink != nil {
			servers := operation.Shrink.Servers
			if len(servers) > 0 {
				fields[FieldNodeIP] = servers[0].AdvertiseIP
				fields[FieldNodeHostname] = servers[0].Hostname
				fields[FieldNodeRole] = servers[0].Role
			}
		}
	case ops.OperationUpdate:
		if operation.Update != nil {
			locator, err := loc.ParseLocator(operation.Update.UpdatePackage)
			if err != nil {
				return fields, trace.Wrap(err)
			}
			fields[FieldName] = locator.Name
			fields[FieldVersion] = locator.Version
		}
	}
	return fields, nil
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
	// FieldName contains name, e.g. resource name, application name, etc.
	FieldName = "name"
	// FieldCluster contains name of the cluster that generated an event.
	FieldCluster = "cluster"
	// FieldKind contains resource kind.
	FieldKind = "kind"
	// FieldUser contains name of the user who triggered an event.
	FieldUser = "user"
	// FieldOwner contains name of the user a resource belongs to.
	FieldOwner = "owner"
	// FieldReleaseName contains application release name.
	FieldReleaseName = "releaseName"
	// FieldVersion contains application package version.
	FieldVersion = "version"
	// FieldReason contains cluster deactivation reason.
	FieldReason = "reason"
	// FieldTime contains event time.
	FieldTime = "time"
	// FieldRoles contains roles of a new user.
	FieldRoles = "roles"
)
