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

const (
	// SiteLabelName defines the name of the cluster name label
	SiteLabelName = "Name"
	// SystemRepository is the system package repository
	SystemRepository = "gravitational.io"
	// ProviderGeneric is the generic cluster infrastructure provider
	ProviderGeneric = "generic"
	// ProgressStateCompleted signifies the operation completed progress value
	ProgressStateCompleted = "completed"
	// ProgressStateInProgress signifies the operation in-progress progress value
	ProgressStateInProgress = "in_progress"
	// ProgressStateFailed signifies the operation failed progress value
	ProgressStateFailed = "failed"
	// ServiceAccountTokenSecretType defines the secret type for service account tokens
	//nolint:gosec // not a hardcoded credential
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
	// SiteStateUpdatingEnviron is the state of the cluster when it's updating runtime environment variables on nodes
	SiteStateUpdatingEnviron = "updating_cluster_environ"
	// SiteStateUpdatingConfig is the state of the cluster when it's updating configuration
	SiteStateUpdatingConfig = "updating_cluster_config"
	// SiteStateReconfiguring is the state of the cluster when its advertise IP is being reconfigured
	SiteStateReconfiguring = "reconfiguring"
	// SiteStateDegraded means that the application installed on a deployed site is failing its health check
	SiteStateDegraded = "degraded"
	// SiteStateOffline means that OpsCenter cannot connect to remote site
	SiteStateOffline = "offline"

	// OperationInstall identifies the install operation
	OperationInstall = "operation_install"
	// OperationStateInstallInitiated signifies the install operation initiated state
	OperationStateInstallInitiated = "install_initiated"
	// OperationStateInstallPrechecks signifies the install operation prechecks state
	OperationStateInstallPrechecks = "install_prechecks"
	// OperationStateInstallProvisioning signifies the install operation provisioning state
	OperationStateInstallProvisioning = "install_provisioning"
	// OperationStateInstallDeploying signifies the install operation deploying state
	OperationStateInstallDeploying = "install_deploying"

	// OperationReconfigure is the name of the operation that reconfigures
	// the cluster advertise IP.
	OperationReconfigure = "operation_reconfigure"
	// OperationReconfigureInProgress is the operation state indicating
	// cluster advertise IP is being reconfigured.
	OperationReconfigureInProgress = "reconfigure_in_progress"

	// OperationStateReady indicates that the operation is ready to
	// be executed by the installer process
	OperationStateReady = "ready"

	// OperationExpand identifies the expand operation
	OperationExpand = "operation_expand"
	// OperationStateExpandInitiated defines the expand operation initiated state
	OperationStateExpandInitiated = "expand_initiated"
	// OperationStateExpandPrechecks defines the expand operation prechecks state
	OperationStateExpandPrechecks = "expand_prechecks"
	// OperationStateExpandProvisioning defines the expand operation provisioning state
	OperationStateExpandProvisioning = "expand_provisioning"
	// OperationStateExpandDeploying defines the expand operation deploying state
	OperationStateExpandDeploying = "expand_deploying"

	// OperationUpdate identifies the update operation
	OperationUpdate = "operation_update"
	// OperationStateUpdateInProgress defines the update operation in-progress state
	OperationStateUpdateInProgress = "update_in_progress"

	// OperationShrink identifies the shrink operation
	OperationShrink = "operation_shrink"
	// OperationStateShrinkInProgress defines the shrink operation in-progress state
	OperationStateShrinkInProgress = "shrink_in_progress"

	// OperationUninstall identifies the uninstall operation
	OperationUninstall = "operation_uninstall"
	// OperationStateUninstallInProgress defines the uninstall operation in-progress state
	OperationStateUninstallInProgress = "uninstall_in_progress"

	// OperationGarbageCollect identifies the gc operation
	OperationGarbageCollect = "operation_gc"
	// OperationGarbageCollectInProgress defines the gc operation in-progress state
	OperationGarbageCollectInProgress = "gc_in_progress"

	// OperationUpdateRuntimeEnviron identifies the runtime environment update operation
	OperationUpdateRuntimeEnviron = "operation_update_environ"
	// OperationUpdateRuntimeEnvironInProgress defines the runtime environment update operation in-progress state
	OperationUpdateRuntimeEnvironInProgress = "update_environ_in_progress"

	// OperationUpdateConfig identifies the cluster configuration update operation
	OperationUpdateConfig = "operation_update_config"
	// OperationUpdateConfigInProgress defines the cluster configuration update operation in-progress state
	OperationUpdateConfigInProgress = "update_config_in_progress"

	// Common operation states

	// OperationStateCompleted signifies a completed operation
	OperationStateCompleted = "completed"
	// OperationStateFailed signifies a failed operation
	OperationStateFailed = "failed"

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
)

var (
	// OperationStartedToClusterState defines states the cluster transitions
	// into when a certain operation starts
	OperationStartedToClusterState = map[string]string{
		OperationInstall:              SiteStateInstalling,
		OperationExpand:               SiteStateExpanding,
		OperationUpdate:               SiteStateUpdating,
		OperationShrink:               SiteStateShrinking,
		OperationUninstall:            SiteStateUninstalling,
		OperationGarbageCollect:       SiteStateGarbageCollecting,
		OperationUpdateRuntimeEnviron: SiteStateUpdatingEnviron,
		OperationUpdateConfig:         SiteStateUpdatingConfig,
		OperationReconfigure:          SiteStateReconfiguring,
	}

	// OperationSucceededToClusterState defines states the cluster transitions
	// into when a certain operation completes successfully
	OperationSucceededToClusterState = map[string]string{
		OperationInstall:              SiteStateActive,
		OperationExpand:               SiteStateActive,
		OperationUpdate:               SiteStateActive,
		OperationShrink:               SiteStateActive,
		OperationUninstall:            SiteStateNotInstalled,
		OperationGarbageCollect:       SiteStateActive,
		OperationUpdateRuntimeEnviron: SiteStateActive,
		OperationUpdateConfig:         SiteStateActive,
		OperationReconfigure:          SiteStateActive,
	}

	// OperationFailedToClusterState defines states the cluster transitions
	// into when a certain operation fails.
	// If an state transition for a specific operation is missing, the cluster
	// state is left unchanged
	OperationFailedToClusterState = map[string]string{
		OperationInstall:              SiteStateFailed,
		OperationExpand:               SiteStateActive,
		OperationUpdate:               SiteStateUpdating,
		OperationShrink:               SiteStateActive,
		OperationUninstall:            SiteStateFailed,
		OperationGarbageCollect:       SiteStateActive,
		OperationUpdateRuntimeEnviron: SiteStateUpdatingEnviron,
		OperationUpdateConfig:         SiteStateUpdatingConfig,
		OperationReconfigure:          SiteStateFailed,
	}
)
