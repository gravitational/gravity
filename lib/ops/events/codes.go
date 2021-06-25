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

import "github.com/gravitational/teleport/lib/events"

var (
	// OperationInstallStart is emitted when a cluster installation starts.
	OperationInstallStart = events.Event{
		Name: OperationStartedEvent,
		Code: OperationInstallStartCode,
	}
	// OperationInstallComplete is emitted when a cluster installation successfully completes.
	OperationInstallComplete = events.Event{
		Name: OperationCompletedEvent,
		Code: OperationInstallCompleteCode,
	}
	// OperationInstallFailure is emitted when a cluster installation fails.
	OperationInstallFailure = events.Event{
		Name: OperationFailedEvent,
		Code: OperationInstallFailureCode,
	}
	// OperationExpandStart is emitted when a new node starts joining the cluster.
	OperationExpandStart = events.Event{
		Name: OperationStartedEvent,
		Code: OperationExpandStartCode,
	}
	// OperationExpandComplete is emitted when a node has successfully joined the cluster.
	OperationExpandComplete = events.Event{
		Name: OperationCompletedEvent,
		Code: OperationExpandCompleteCode,
	}
	// OperationExpandFailure is emitted when a node fails to join the cluster.
	OperationExpandFailure = events.Event{
		Name: OperationFailedEvent,
		Code: OperationExpandFailureCode,
	}
	// OperationShrinkStart is emitted when a node is leaving the cluster.
	OperationShrinkStart = events.Event{
		Name: OperationStartedEvent,
		Code: OperationShrinkStartCode,
	}
	// OperationShrinkComplete is emitted when a node has left the cluster.
	OperationShrinkComplete = events.Event{
		Name: OperationCompletedEvent,
		Code: OperationShrinkCompleteCode,
	}
	// OperationShrinkFailure is emitted when a node fails to leave the cluster.
	OperationShrinkFailure = events.Event{
		Name: OperationFailedEvent,
		Code: OperationShrinkFailureCode,
	}
	// OperationUpdateStart is emitted when cluster upgrade is started.
	OperationUpdateStart = events.Event{
		Name: OperationStartedEvent,
		Code: OperationUpdateStartCode,
	}
	// OperationUpdateComplete is emitted when cluster upgrade successfully finishes.
	OperationUpdateComplete = events.Event{
		Name: OperationCompletedEvent,
		Code: OperationUpdateCompleteCode,
	}
	// OperationUpdateFailure is emitted when cluster upgrade fails.
	OperationUpdateFailure = events.Event{
		Name: OperationFailedEvent,
		Code: OperationUpdateFailureCode,
	}
	// OperationUninstallStart is emitted when cluster uninstall is launched.
	OperationUninstallStart = events.Event{
		Name: OperationStartedEvent,
		Code: OperationUninstallStartCode,
	}
	// OperationUninstallComplete is emitted when cluster has been uninstalled.
	OperationUninstallComplete = events.Event{
		Name: OperationCompletedEvent,
		Code: OperationUninstallCompleteCode,
	}
	// OperationUninstallFailure is emitted when cluster uninstall fails.
	OperationUninstallFailure = events.Event{
		Name: OperationFailedEvent,
		Code: OperationUninstallFailureCode,
	}
	// OperationGCStart is emitted when garbage collection is started on a cluster.
	OperationGCStart = events.Event{
		Name: OperationStartedEvent,
		Code: OperationGCStartCode,
	}
	// OperationGCComplete is emitted when cluster garbage collection successfully completes.
	OperationGCComplete = events.Event{
		Name: OperationCompletedEvent,
		Code: OperationGCCompleteCode,
	}
	// OperationGCFailure is emitted when cluster garbage collection fails.
	OperationGCFailure = events.Event{
		Name: OperationFailedEvent,
		Code: OperationGCFailureCode,
	}
	// OperationEnvStart is emitted when cluster runtime environment update is launched.
	OperationEnvStart = events.Event{
		Name: OperationStartedEvent,
		Code: OperationEnvStartCode,
	}
	// OperationEnvComplete is emitted when cluster runtime environment update successfully completes.
	OperationEnvComplete = events.Event{
		Name: OperationCompletedEvent,
		Code: OperationEnvCompleteCode,
	}
	// OperationEnvFailure is emitted when cluster runtime environment update fails.
	OperationEnvFailure = events.Event{
		Name: OperationFailedEvent,
		Code: OperationEnvFailureCode,
	}
	// OperationConfigStart is emitted when cluster configuration update launches.
	OperationConfigStart = events.Event{
		Name: OperationStartedEvent,
		Code: OperationConfigStartCode,
	}
	// OperationConfigComplete is emitted when cluster configuration update successfully completes.
	OperationConfigComplete = events.Event{
		Name: OperationCompletedEvent,
		Code: OperationConfigCompleteCode,
	}
	// OperationConfigFailure is emitted when cluster configuration update fails.
	OperationConfigFailure = events.Event{
		Name: OperationFailedEvent,
		Code: OperationConfigFailureCode,
	}
	// OperationReconfigureStart is emitted when advertise IP is being reconfigured.
	OperationReconfigureStart = events.Event{
		Name: OperationStartedEvent,
		Code: OperationReconfigureStartCode,
	}
	// OperationReconfigureComplete is emitted when advertise IP reconfiguration completes.
	OperationReconfigureComplete = events.Event{
		Name: OperationCompletedEvent,
		Code: OperationReconfigureCompleteCode,
	}
	// OperationReconfigureFailure is emitted when advertise IP reconfiguration fails.
	OperationReconfigureFailure = events.Event{
		Name: OperationFailedEvent,
		Code: OperationReconfigureFailureCode,
	}
	// UserCreated is emitted when a user is created/updated.
	UserCreated = events.Event{
		Name: UserCreatedEvent,
		Code: UserCreatedCode,
	}
	// UserDeleted is emitted when a user is deleted.
	UserDeleted = events.Event{
		Name: UserDeletedEvent,
		Code: UserDeletedCode,
	}
	// TokenCreated is emitted when a token is created/updated.
	TokenCreated = events.Event{
		Name: TokenCreatedEvent,
		Code: TokenCreatedCode,
	}
	// TokenDeleted is emitted when a token is deleted.
	TokenDeleted = events.Event{
		Name: TokenDeletedEvent,
		Code: TokenDeletedCode,
	}
	// GithubConnectorCreated is emitted when a Github connector is created/updated.
	GithubConnectorCreated = events.Event{
		Name: GithubConnectorCreatedEvent,
		Code: GithubConnectorCreatedCode,
	}
	// GithubConnectorDeleted is emitted when a Github connector is deleted.
	GithubConnectorDeleted = events.Event{
		Name: GithubConnectorDeletedEvent,
		Code: GithubConnectorDeletedCode,
	}
	// LogForwarderCreated is emitted when a log forwarder is created/updated.
	LogForwarderCreated = events.Event{
		Name: LogForwarderCreatedEvent,
		Code: LogForwarderCreatedCode,
	}
	// LogForwarderDeleted is emitted when a log forwarder is deleted.
	LogForwarderDeleted = events.Event{
		Name: LogForwarderDeletedEvent,
		Code: LogForwarderDeletedCode,
	}
	// TLSKeyPairCreated is emitted when cluster web certificate is updated.
	TLSKeyPairCreated = events.Event{
		Name: TLSKeyPairCreatedEvent,
		Code: TLSKeyPairCreatedCode,
	}
	// TLSKeyPairDeleted is emitted when cluster web certificate is deleted.
	TLSKeyPairDeleted = events.Event{
		Name: TLSKeyPairDeletedEvent,
		Code: TLSKeyPairDeletedCode,
	}
	// AuthPreferenceUpdated is emitted when cluster auth preference is updated.
	AuthPreferenceUpdated = events.Event{
		Name: AuthPreferenceUpdatedEvent,
		Code: AuthPreferenceUpdatedCode,
	}
	// SMTPConfigCreated is emitted when SMTP configuration is created/updated.
	SMTPConfigCreated = events.Event{
		Name: SMTPConfigCreatedEvent,
		Code: SMTPConfigCreatedCode,
	}
	// SMTPConfigDeleted is emitted when SMTP configuration is deleted.
	SMTPConfigDeleted = events.Event{
		Name: SMTPConfigDeletedEvent,
		Code: SMTPConfigDeletedCode,
	}
	// AlertCreated is emitted when monitoring alert is created/updated.
	AlertCreated = events.Event{
		Name: AlertCreatedEvent,
		Code: AlertCreatedCode,
	}
	// AlertDeleted is emitted when monitoring alert is deleted.
	AlertDeleted = events.Event{
		Name: AlertDeletedEvent,
		Code: AlertDeletedCode,
	}
	// AlertTargetCreated is emitted when monitoring alert target is created/updated.
	AlertTargetCreated = events.Event{
		Name: AlertTargetCreatedEvent,
		Code: AlertTargetCreatedCode,
	}
	// AlertTargetDeleted is emitted when monitoring alert target is deleted.
	AlertTargetDeleted = events.Event{
		Name: AlertTargetDeletedEvent,
		Code: AlertTargetDeletedCode,
	}
	// AuthGatewayUpdated is emitted when cluster auth gateway settings are updated.
	AuthGatewayUpdated = events.Event{
		Name: AuthGatewayUpdatedEvent,
		Code: AuthGatewayUpdatedCode,
	}
	// UserInviteCreated is emitted when a user invite is created.
	UserInviteCreated = events.Event{
		Name: InviteCreatedEvent,
		Code: UserInviteCreatedCode,
	}
	// PersistentStorageUpdated is emitted when persistent storage configuration is updated.
	PersistentStorageUpdated = events.Event{
		Name: PersistentStorageUpdatedEvent,
		Code: PersistentStorageUpdatedCode,
	}
	// ClusterUnhealthy is emitted when cluster becomes unhealthy.
	ClusterUnhealthy = events.Event{
		Name: ClusterDegradedEvent,
		Code: ClusterUnhealthyCode,
	}
	// ClusterHealthy is emitted when cluster becomes healthy.
	ClusterHealthy = events.Event{
		Name: ClusterActivatedEvent,
		Code: ClusterHealthyCode,
	}
	// ApplicationInstall is emitted when a new application image is installed.
	ApplicationInstall = events.Event{
		Name: AppInstalledEvent,
		Code: ApplicationInstallCode,
	}
	// ApplicationUpgrade is emitted when an application release is upgraded.
	ApplicationUpgrade = events.Event{
		Name: AppUpgradedEvent,
		Code: ApplicationUpgradeCode,
	}
	// ApplicationRollback is emitted when an application release is rolled back.
	ApplicationRollback = events.Event{
		Name: AppRolledBackEvent,
		Code: ApplicationRollbackCode,
	}
	// ApplicationUninstall is emitted when an application release is uninstalled.
	ApplicationUninstall = events.Event{
		Name: AppUninstalledEvent,
		Code: ApplicationUninstallCode,
	}
)

// There is no strict algorithm for picking an event code, however existing
// event codes are currently loosely categorized as follows:
//
//  * Operation related events are in `00xx` group. Events that are related to
//    the same operation type but represent different outcome (success/failure)
//    have the same numeric code and respective suffix.
//
//  * Events that represent an action on a resource (or just an object in
//    general, for instance: user, role, connector, etc.) are in `1xxx` (for
//    "create") and `2xxx` (for "delete") group respectively.
//
//  * Events that represent a cluster-wide action (cluster status changes,
//    license expires, etc.) are in `3xxx` group.
//
//  * Application catalog related events are in `4xxx` group.
const (
	// OpereationInstallStartCode is the install operation start event code.
	OperationInstallStartCode = "G0001I"
	// OperationInstallCompleteCode is the install operation complete event code.
	OperationInstallCompleteCode = "G0002I"
	// OperationInstallFailureCode is the install operation failure event code.
	OperationInstallFailureCode = "G0002E"
	// OperationExpandStartCode is the expand operation start event code.
	OperationExpandStartCode = "G0003I"
	// OperationExpandCompleteCode is the expand operation complete event code.
	OperationExpandCompleteCode = "G0004I"
	// OperationExpandFailureCode is the expand operation failure event code.
	OperationExpandFailureCode = "G0004E"
	// OperationShrinkStartCide is the shrink operation start event code.
	OperationShrinkStartCode = "G0005I"
	// OperationShrinkCompleteCode is the shrink operation complete event code.
	OperationShrinkCompleteCode = "G0006I"
	// OperationShrinkFailureCode is the shrink operation failure event code.
	OperationShrinkFailureCode = "G0006E"
	// OperationUpdateStartCode is the update operation start event code.
	OperationUpdateStartCode = "G0007I"
	// OperationUpdateCompeteCode is the update operation complete event code.
	OperationUpdateCompleteCode = "G0008I"
	// OperationUpdateFailureCode is the update operation failure event code.
	OperationUpdateFailureCode = "G0008E"
	// OperationUninstallStartCode is the uninstall operation start event code.
	OperationUninstallStartCode = "G0009I"
	// OperationUninstallCompleteCode is the uninstall operation complete event code.
	OperationUninstallCompleteCode = "G0010I"
	// OperationUninstallFailureCode is the uninstall operation failure event code.
	OperationUninstallFailureCode = "G0010E"
	// OperationGCStartCode is the garbage collect operation start event code.
	OperationGCStartCode = "G0011I"
	// OperationGCCompleteCode is the garbage collect operation complete event code.
	OperationGCCompleteCode = "G0012I"
	// OperationGCFailureCode is the garbage collect operation failure event code.
	OperationGCFailureCode = "G0012E"
	// OperationEnvStartCode is the runtime environment update operation start event code.
	OperationEnvStartCode = "G0013I"
	// OperationEnvCompleteCode is the runtime environment update operation complete event code.
	OperationEnvCompleteCode = "G0014I"
	// OperationEnvFailureCode is the runtime environment update operation failure event code.
	OperationEnvFailureCode = "G0014E"
	// OperationConfigStartCode is the cluster configuration update operation start event code.
	OperationConfigStartCode = "G0015I"
	// OperationConfigCompleteCode is the cluster configuration update operation complete event code.
	OperationConfigCompleteCode = "G0016I"
	// OperationConfigFailureCode is the cluster configuration update operation failure event code.
	OperationConfigFailureCode = "G0016E"
	// OperationReconfigureStartCode is the cluster reconfigure operation start event code.
	OperationReconfigureStartCode = "G0017I"
	// OperationReconfigureCompleteCode is the cluster reconfigure operation complete event code.
	OperationReconfigureCompleteCode = "G0018I"
	// OperationReconfigureFailureCode is the cluster reconfigure operation failure event code.
	OperationReconfigureFailureCode = "G0018E"
	// UserCreatedCode is the user created event code.
	UserCreatedCode = "G1000I"
	// UserDeletedCode is the user deleted event code.
	UserDeletedCode = "G2000I"
	// TokenCreatedCode is the user token created event code.
	TokenCreatedCode = "G1001I"
	// TokenDeletedCode is the user token deleted event code.
	TokenDeletedCode = "G2001I"
	// GithubConnectorCreatedCode is the Github connector created event code.
	GithubConnectorCreatedCode = "G1002I"
	// GithubConnectorDeletedCode is the Github connector deleted event code.
	GithubConnectorDeletedCode = "G2002I"
	// LogForwarderCreatedCode is the log forwarder created event code.
	LogForwarderCreatedCode = "G1003I"
	// LogForwarderDeletedCode is the log forwarder deleted event code.
	LogForwarderDeletedCode = "G2003I"
	// TLSKeyPairCreatedCode is the TLS key pair created event code.
	TLSKeyPairCreatedCode = "G1004I"
	// TLSKeyPairDeletedCode is the TLS key pair deleted event code.
	TLSKeyPairDeletedCode = "G2004I"
	// AuthPreferenceUpdatedCode is the cluster auth preference updated event code.
	AuthPreferenceUpdatedCode = "G1005I"
	// SMTPConfigCreatedCode is the SMTP configuration updated event code.
	SMTPConfigCreatedCode = "G1006I"
	// SMTPConfigDeletedCode is the SMTP configuration deleted event code.
	SMTPConfigDeletedCode = "G2006I"
	// AlertCreatedCode is the monitoring alert created event code.
	AlertCreatedCode = "G1007I"
	// AlertDeletedCode is the monitoring alert deleted event code.
	AlertDeletedCode = "G2007I"
	// AlertTargetCreatedCode is the monitoring alert target created event code.
	AlertTargetCreatedCode = "G1008I"
	// AlertTargetDeletedCode is the monitoring alert target deleted event code.
	AlertTargetDeletedCode = "G2008I"
	// AuthGatewayUpdatedCode is the auth gateway updated event code.
	AuthGatewayUpdatedCode = "G1009I"
	// UserInviteCreatedCode is the user invite created event code.
	UserInviteCreatedCode = "G1010I"
	// PersistentStorageUpdatedCode is the persistent storage updated event code.
	PersistentStorageUpdatedCode = "G1011I"
	// ClusterUnhealthyCode is the cluster goes unhealthy event code.
	ClusterUnhealthyCode = "G3000W"
	// ClusterHealthyCode is the cluster goes healthy event code.
	ClusterHealthyCode = "G3001I"
	// ApplicationInstallCode is the application release install event code.
	ApplicationInstallCode = "G4000I"
	// ApplicationUpgradeCode is the application release upgrade event code.
	ApplicationUpgradeCode = "G4001I"
	// ApplicationRollbackCode is the application release rollback event code.
	ApplicationRollbackCode = "G4002I"
	// ApplicationUninstallCode is the application release uninstall event code.
	ApplicationUninstallCode = "G4003I"
)

const (
	// OperationStartedEvent fires when an operation starts.
	OperationStartedEvent = "operation.started"
	// OperationCompletedEvent fires when an operation completes successfully.
	OperationCompletedEvent = "operation.completed"
	// OperationFailedEvent fires when an operation completes with error.
	OperationFailedEvent = "operation.failed"

	// AppInstalledEvent fires when an application image is installed.
	AppInstalledEvent = "application.installed"
	// AppUpgradedEvent fires when an application release is upgraded.
	AppUpgradedEvent = "application.upgraded"
	// AppRolledBackEvent fires when an application release is rolled back.
	AppRolledBackEvent = "application.rolledback"
	// AppUninstalledEvent fires when an application release is uninstalled.
	AppUninstalledEvent = "application.uninstalled"

	// UserCreatedEvent fires when a user is created/updated.
	UserCreatedEvent = "user.created"
	// UserDeletedEvent fires when a user is deleted.
	UserDeletedEvent = "user.deleted"
	// TokenCreatedEvent fires when a token is created/updated.
	TokenCreatedEvent = "token.created"
	// TokenDeletedEvent fires when a token is deleted.
	TokenDeletedEvent = "token.deleted"
	// GithubConnectorCreatedEvent fires when a Github connector is created/updated.
	GithubConnectorCreatedEvent = "github.created"
	// GithubConnectorDeletedEvent fires when a Github connector is deleted.
	GithubConnectorDeletedEvent = "github.deleted"
	// LogForwarderCreatedEvent fires when a log forwarder is created/updated.
	LogForwarderCreatedEvent = "logforwarder.created"
	// LogForwarderDeletedEvent fires when a log forwarder is deleted.
	LogForwarderDeletedEvent = "logforwarder.delete"
	// TLSKeyPairCreatedEvent fires when a TLS key pair is created/updated.
	TLSKeyPairCreatedEvent = "tlskeypair.created"
	// TLSKeyPairDeletedEvent fires when a TLS key pair is deleted.
	TLSKeyPairDeletedEvent = "tlskeypair.deleted"
	// AuthPreferenceUpdatedEvent fires when cluster auth preference is updated.
	AuthPreferenceUpdatedEvent = "authpreference.updated"
	// SMTPConfigCreatedEvent fires when SMTP config is created/updated.
	SMTPConfigCreatedEvent = "smtpconfig.created"
	// SMTPConfigDeletedEvent fires when SMTP config is deleted.
	SMTPConfigDeletedEvent = "smtpconfig.deleted"
	// AlertCreatedEvent fires when monitoring alert is created/updated.
	AlertCreatedEvent = "alert.created"
	// AlertDeletedEvent fires when monitoring alert is deleted.
	AlertDeletedEvent = "alert.deleted"
	// AlertTargetCreatedEvent fires when monitoring alert target is created/updated.
	AlertTargetCreatedEvent = "alerttarget.created"
	// AlertTargetDeletedEvent fires when monitoring alert target is deleted.
	AlertTargetDeletedEvent = "alerttarget.deleted"
	// AuthGatewayUpdatedEvent fires when auth gateway settings are updated.
	AuthGatewayUpdatedEvent = "authgateway.updated"
	// InviteCreatedEvent fires when a new user invitation is generated.
	InviteCreatedEvent = "invite.created"
	// PersistentStorageUpdatedEvent fires when persistent storage configuration is updated.
	PersistentStorageUpdatedEvent = "persistentstorage.updated"

	// ClusterDegradedEvent fires when cluster health check fails.
	ClusterDegradedEvent = "cluster.degraded"
	// ClusterActivatedEvent fires when cluster becomes healthy again.
	ClusterActivatedEvent = "cluster.activated"
)
