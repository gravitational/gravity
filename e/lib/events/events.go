// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package events

import (
	"github.com/gravitational/teleport/lib/events"
)

var (
	// RoleCreated is emitted when a role is created/updated.
	RoleCreated = events.Event{
		Name: RoleCreatedEvent,
		Code: RoleCreatedCode,
	}
	// RoleDeleted is emitted when a role is deleted.
	RoleDeleted = events.Event{
		Name: RoleDeletedEvent,
		Code: RoleDeletedCode,
	}
	// OIDCConnectorCreated is emitted when an OIDC connector is created/updated.
	OIDCConnectorCreated = events.Event{
		Name: OIDCConnectorCreatedEvent,
		Code: OIDCConnectorCreatedCode,
	}
	// OIDCConnectorDeleted is emitted when an OIDC connector is deleted.
	OIDCConnectorDeleted = events.Event{
		Name: OIDCConnectorDeletedEvent,
		Code: OIDCConnectorDeletedCode,
	}
	// SAMLConnectorCreated is emitted when a SAML connector is created/updated.
	SAMLConnectorCreated = events.Event{
		Name: SAMLConnectorCreatedEvent,
		Code: SAMLConnectorCreatedCode,
	}
	// SAMLConnectorDeleted is emitted when a SAML connector is deleted.
	SAMLConnectorDeleted = events.Event{
		Name: SAMLConnectorDeletedEvent,
		Code: SAMLConnectorDeletedCode,
	}
	// EndpointsUpdated is emitted when Gravity Hub endpoints are created/updated.
	EndpointsUpdated = events.Event{
		Name: EndpointsUpdatedEvent,
		Code: EndpointsUpdatedCode,
	}
	// RemoteSupportEnabled is emitted when remote support is turned on.
	RemoteSupportEnabled = events.Event{
		Name: RemoteSupportEnabledEvent,
		Code: RemoteSupportEnabledCode,
	}
	// RemoteSupportDisabled is emitted when remote support is turned off.
	RemoteSupportDisabled = events.Event{
		Name: RemoteSupportDisabledEvent,
		Code: RemoteSupportDisabledCode,
	}
	// LicenseGenerated is emitted when a new license is generated.
	LicenseGenerated = events.Event{
		Name: LicenseGeneratedEvent,
		Code: LicenseGeneratedCode,
	}
	// LicenseExpired is emitted when cluster license expires.
	LicenseExpired = events.Event{
		Name: LicenseExpiredEvent,
		Code: LicenseExpiredCode,
	}
	// LicenseUpdated is emitted when cluster license is updated.
	LicenseUpdated = events.Event{
		Name: LicenseUpdatedEvent,
		Code: LicenseUpdatedCode,
	}
	// UpdatesEnabled is emitted when periodic updates are turned on.
	UpdatesEnabled = events.Event{
		Name: UpdatesEnabledEvent,
		Code: UpdatesEnabledCode,
	}
	// UpdatesDisabled is emitted when periodic updates are turned off.
	UpdatesDisabled = events.Event{
		Name: UpdatesDisabledEvent,
		Code: UpdatesDisabledCode,
	}
	// UpdatesDownloaded is emitted when periodic updates download a new version.
	UpdatesDownloaded = events.Event{
		Name: UpdatesDownloadedEvent,
		Code: UpdatesDownloadedCode,
	}
)

// When picking a new event code, refer to the suggestions in
// lib/ops/events/codes.go in the open-source repo.
const (
	// RoleCreatedCode is the role created event code.
	RoleCreatedCode = "GE1000I"
	// RoleDeletedCode is the role deleted event code.
	RoleDeletedCode = "GE2000I"
	// OIDCConnectorCreatedCode is the OIDC connector created event code.
	OIDCConnectorCreatedCode = "GE1001I"
	// OIDCConnectorDeletedCode is the OIDC connector deleted event code.
	OIDCConnectorDeletedCode = "GE2001I"
	// SAMLConnectorCreatedCode is the SAML connector created event code.
	SAMLConnectorCreatedCode = "GE1002I"
	// SAMLConnectorDeletedCode is the SAML connector deleted event code.
	SAMLConnectorDeletedCode = "GE2002I"
	// EndpointsUpdatedCode is the endpoints created event code.
	EndpointsUpdatedCode = "GE1003I"
	// RemoteSupportEnabledCode is the remote support turned on event code.
	RemoteSupportEnabledCode = "GE3000I"
	// RemoteSupportDisabledCode is the remote support turned off event code.
	RemoteSupportDisabledCode = "GE3001I"
	// LicenseGeneratedCode is the license generated event code.
	LicenseGeneratedCode = "GE3002I"
	// LicenseExpiredCode is the license expired event code.
	LicenseExpiredCode = "GE3003I"
	// LicenseUpdatedCode is the license updated event code.
	LicenseUpdatedCode = "GE3004I"
	// UpdatesEnabledCode is the periodic updates turned on event code.
	UpdatesEnabledCode = "GE3005I"
	// UpdatesDisabledCode is the periodic updates turned off event code.
	UpdatesDisabledCode = "GE3006I"
	// UpdatesDownloadedCode is the new version downloaded event code.
	UpdatesDownloadedCode = "GE3007I"
)

const (
	// RoleCreatedEvent fires when role is created/updated.
	RoleCreatedEvent = "role.created"
	// RoleDeletedEvent fires when role is deleted.
	RoleDeletedEvent = "role.deleted"
	// OIDCConnectorCreatedEvent fires when OIDC connector is created/updated.
	OIDCConnectorCreatedEvent = "oidc.created"
	// OIDCConnectorDeletedEvent fires when OIDC connector is deleted.
	OIDCConnectorDeletedEvent = "oidc.deleted"
	// SAMLConnectorCreatedEvent fires when SAML connector is created/updated.
	SAMLConnectorCreatedEvent = "saml.created"
	// SAMLConnectorDeletedEvent fires when SAML connector is deleted.
	SAMLConnectorDeletedEvent = "saml.deleted"
	// EndpointsUpdatedEvent fires when Gravity Hub endpoints are updated.
	EndpointsUpdatedEvent = "endpoints.updated"

	// RemoteSupportEnabledEvent fires when cluster enables remote support with an Gravity Hub.
	RemoteSupportEnabledEvent = "remotesupport.enabled"
	// RemoteSupportDisabledEvent fires when cluster disables Gravity Hub remote support.
	RemoteSupportDisabledEvent = "remotesupport.disabled"

	// UpdatesEnabledEvent fires when periodic updates are turned on.
	UpdatesEnabledEvent = "periodicupdates.enabled"
	// UpdatesDisabledEvent fires when periodic updates are turned off.
	UpdatesDisabledEvent = "periodicupdates.disabled"
	// UpdatesDownloadedEvent fires when periodic updates download an update package.
	UpdatesDownloadedEvent = "periodicupdates.downloaded"

	// LicenseExpiredEvent fires when cluster license expires.
	LicenseExpiredEvent = "license.expired"
	// LicenseUpdatedEvent fires when cluster license is updated.
	LicenseUpdatedEvent = "license.updated"
	// LicenseGeneratedEvent fires when an Gravity Hub generates a license.
	LicenseGeneratedEvent = "license.generated"
)

const (
	// FieldOpsCenter contains Gravity Hub name.
	FieldOpsCenter = "hub"
	// FieldInterval contains time interval, e.g. for periodic updates.
	FieldInterval = "interval"
	// FieldExpires contains expiration time, e.g. for license.
	FieldExpires = "expires"
	// FieldMaxNodes contains license nodes limit.
	FieldMaxNodes = "maxNodes"
)
