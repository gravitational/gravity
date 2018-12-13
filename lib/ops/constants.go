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

/*

Site state transitions
----------------------

1. Site states transition schema during install

created ->
install_initiated ->
   only if on prem provisioners:
      install_setting_plan ->
      instal_plan_set ->
install_provisioning ->
install_provisioned ->
install_deploying ->
  (if ok ) -> active
  (if failed during any of the stages) -> failed

2. Site states transition during uninstall

uninstall_in_progress ->
  (if ok ) -> created
  (if failed during any of the stages) -> failed


Progress indicator transitions
-------------------------------

in_progress ->
   failed
   or
   completed

*/
package ops

import (
	"time"
)

const (
	SiteLabelName                 = "Name"
	SystemRepository              = "gravitational.io"
	ProviderGeneric               = "generic"
	TeleportProxyAddress          = "teleport_proxy_address"
	ProgressStateCompleted        = "completed"
	ProgressStateInProgress       = "in_progress"
	ProgressStateFailed           = "failed"
	InstallTerminated             = "terminated"
	ServiceAccountTokenSecretType = "kubernetes.io/service-account-token"

	// SiteStateNotInstalled is a state where a site has just been created or uninstalled and
	// no active operation for it is in progress
	SiteStateNotInstalled = "not_installed"
	// SiteStateFailed indicates that the site is in an invalid state, e.g. its installation
	// or uninstallation failed
	SiteStateFailed = "failed"
	// SiteStateActive means that a site is properly deployed and its application is functional
	SiteStateActive = "active"
	// SiteStateInstalling means that the site is being installed
	SiteStateInstalling = "installing"
	// SiteStateUpdating means that there's an update operation in progress
	SiteStateUpdating = "updating"
	// SiteStateExpanding means that the site is being expanded
	SiteStateExpanding = "expanding"
	// SiteStateShrinking means that the site is being shrunk
	SiteStateShrinking = "shrinking"
	// SiteStateUninstalling means that the site is being uninstalled
	SiteStateUninstalling = "uninstalling"
	// SiteStateGarbageCollecting is the state of the cluster when it's removing unused resources
	SiteStateGarbageCollecting = "collecting_garbage"
	// SiteStateUpdatingEnvars is the state of the cluster when it's updating environment variables on nodes
	SiteStateUpdatingEnvars = "updating_cluster_envars"
	// SiteStateDegraded means that the application installed on a deployed site is failing its health check
	SiteStateDegraded = "degraded"
	// SiteStateOffline means that OpsCenter cannot connect to remote site
	SiteStateOffline = "offline"

	// operation "install" and its states
	OperationInstall                  = "operation_install"
	OperationStateInstallInitiated    = "install_initiated"
	OperationStateInstallPrechecks    = "install_prechecks"
	OperationStateInstallProvisioning = "install_provisioning"
	OperationStateInstallDeploying    = "install_deploying"

	// OperationStateReady indicates that the operation is ready to
	// be executed by the installer process
	OperationStateReady = "ready"

	// operation "expand" and its states
	OperationExpand                  = "operation_expand"
	OperationStateExpandInitiated    = "expand_initiated"
	OperationStateExpandPrechecks    = "expand_prechecks"
	OperationStateExpandProvisioning = "expand_provisioning"
	OperationStateExpandDeploying    = "expand_deploying"

	// operation "update" and its states
	OperationUpdate                = "operation_update"
	OperationStateUpdateInProgress = "update_in_progress"

	// operation "shrink" and its states
	OperationShrink                = "operation_shrink"
	OperationStateShrinkInProgress = "shrink_in_progress"

	// operation "uninstall" and its states
	OperationUninstall                = "operation_uninstall"
	OperationStateUninstallInProgress = "uninstall_in_progress"

	// garbage collection operation
	OperationGarbageCollect           = "operation_gc"
	OperationGarbageCollectInProgress = "gc_in_progress"

	// environment variables update operation
	OperationUpdateEnvars           = "operation_update_envars"
	OperationUpdateEnvarsInProgress = "update_envars_in_progress"

	// common operation states
	OperationStateCompleted = "completed"
	OperationStateFailed    = "failed"

	// Teleport node labels
	// AdvertiseIP defines a label with advertise IP address
	AdvertiseIP = "advertise-ip"
	// ServerFQDN defines a label with FQDN
	ServerFQDN = "fqdn"
	// AppRole defines a label with an application role
	AppRole = "app-role"
	// InstanceType defines a label with a cloud instance type
	InstanceType = "instance-type"
	// Hostname defines a label with hostname
	Hostname = "hostname"

	// TagServiceRole defines a tag used to denote a node role in context of kubernetes
	TagServiceRole = "KubernetesRole"

	// TagKubernetesCluster is a name of the tag containing cluster name AWS resources
	// are usually marked with
	TagKubernetesCluster = "KubernetesCluster"

	// TagRole defines a tag used to denote a node role in the application context
	TagRole = "Role"

	// AgentProvisioner defines the provisioner to the agent.
	// Agent might use specific functionality depending on the set provisioner
	AgentProvisioner = "provisioner"

	// AgentAutoRole defines an agent role that is yet to be determined.
	// The value is used as a role placeholder in the agent download URL in automatic
	// provisioning mode.
	//
	// Currently, provisioner code is responsible for assigning roles to agents
	// based on the following heuristics:
	//  > AWS provisioner uses instance tags to determine the role of an instance
	//
	// Ideally, with an provision script for each role, the assignment should
	// happen in the script by hard-coding a role value into the agent download URL.
	AgentAutoRole = "auto"

	// AgentMode is used to indicate what mode the agent is started in (e.g. shrink)
	AgentMode = "mode"

	// AgentModeShrink means that the agent is started on a node to assist in performing
	// a shrink operation
	AgentModeShrink = "shrink"

	// InstallToken names the query parameter with a one-time install token
	InstallToken = "install_token"

	// AdvertiseAddrParam specifies the name of the agent parameter for advertise address
	AdvertiseAddrParam = "advertise_addr"

	// RetentionDefault is retention policy name for high-res metrics
	RetentionDefault = "default"
	// RetentionMedium is retention policy name for medium-res metrics
	RetentionMedium = "medium"
	// RetentionLong is retention policy name for low-res metrics
	RetentionLong = "long"

	// MaxRetentionDefault is the maximum duration for "default" retention policy
	MaxRetentionDefault = 30 * 24 * time.Hour // ~1 month
	// MaxRetentionMedium is the maximum duration for "medium" retention policy
	MaxRetentionMedium = 6 * 30 * 24 * time.Hour // ~6 months
	// MaxRetentionLong is the maximum duration for "long" retention policy
	MaxRetentionLong = 5 * 365 * 24 * time.Hour // ~5 years
)

var (
	// AllRetentions is a list of names of all retention policies
	AllRetentions = []string{RetentionDefault, RetentionMedium, RetentionLong}

	// RetentionLimits maps retention policy name to its maximum duration
	RetentionLimits = map[string]time.Duration{
		RetentionDefault: MaxRetentionDefault,
		RetentionMedium:  MaxRetentionMedium,
		RetentionLong:    MaxRetentionLong,
	}

	// OperationStartedToClusterState defines states the cluster transitions
	// into when a certain operation starts
	OperationStartedToClusterState = map[string]string{
		OperationInstall:        SiteStateInstalling,
		OperationExpand:         SiteStateExpanding,
		OperationUpdate:         SiteStateUpdating,
		OperationShrink:         SiteStateShrinking,
		OperationUninstall:      SiteStateUninstalling,
		OperationGarbageCollect: SiteStateGarbageCollecting,
		OperationUpdateEnvars:   SiteStateUpdatingEnvars,
	}

	// OperationSucceededToClusterState defines states the cluster transitions
	// into when a certain operation completes successfully
	OperationSucceededToClusterState = map[string]string{
		OperationInstall:        SiteStateActive,
		OperationExpand:         SiteStateActive,
		OperationUpdate:         SiteStateActive,
		OperationShrink:         SiteStateActive,
		OperationUninstall:      SiteStateNotInstalled,
		OperationGarbageCollect: SiteStateActive,
		OperationUpdateEnvars:   SiteStateActive,
	}

	// OperationFailedToClusterState defines states the cluster transitions
	// into when a certain operation fails.
	// If an state transition for a specific operation is missing, the cluster
	// state is left unchanged
	OperationFailedToClusterState = map[string]string{
		OperationInstall:        SiteStateFailed,
		OperationExpand:         SiteStateActive,
		OperationUpdate:         SiteStateUpdating,
		OperationShrink:         SiteStateActive,
		OperationUninstall:      SiteStateFailed,
		OperationGarbageCollect: SiteStateActive,
		OperationUpdateEnvars:   SiteStateActive,
	}
)
