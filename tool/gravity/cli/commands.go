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

package cli

import (
	"net"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/configure"
	"gopkg.in/alecthomas/kingpin.v2"
)

// Application represents the command-line "gravity" application and contains
// definitions of all its flags, arguments and subcommands
type Application struct {
	*kingpin.Application
	// Debug allows to run the command in debug mode
	Debug *bool
	// Silent allows to suppress console output
	Silent *bool
	// Insecure turns off TLS hostname validation
	Insecure *bool
	// StateDir is the local state directory
	StateDir *string
	// EtcdRetryTimeout is the retry timeout for transient etcd errors
	EtcdRetryTimeout *time.Duration
	// UID is the user ID to run the command as
	UID *int
	// GID is the group ID to run the command as
	GID *int
	// ProfileEndpoint allows to enable profiling endpoint
	ProfileEndpoint *string
	// ProfileTo is the location for periodic profiling snapshots
	ProfileTo *string
	// UserLogFile is the path to the user-friendly log file
	UserLogFile *string
	// SystemLogFile is the path to the system log file
	SystemLogFile *string
	// VersionCmd output the binary version
	VersionCmd VersionCmd
	// InstallCmd launches cluster installation
	InstallCmd InstallCmd
	// JoinCmd joins to the installer or existing cluster
	JoinCmd JoinCmd
	// AutoJoinCmd uses cloud provider info to join existing cluster
	AutoJoinCmd AutoJoinCmd
	// LeaveCmd removes the current node from the cluster
	LeaveCmd LeaveCmd
	// RemoveCmd removes the specified node from the cluster
	RemoveCmd RemoveCmd
	// StopCmd stops all gravity services on the node
	StopCmd StopCmd
	// StartCmd starts all gravity services on the node
	StartCmd StartCmd
	// PlanCmd manages an operation plan
	PlanCmd PlanCmd
	// UpdatePlanInitCmd creates a new update operation plan
	UpdatePlanInitCmd UpdatePlanInitCmd
	// PlanDisplayCmd displays plan of an operation
	PlanDisplayCmd PlanDisplayCmd
	// PlanExecuteCmd executes a phase of an active operation
	PlanExecuteCmd PlanExecuteCmd
	// PlanRollbackCmd rolls back a phase of an active operation
	PlanRollbackCmd PlanRollbackCmd
	// PlanSetCmd sets the specified phase state without executing it
	PlanSetCmd PlanSetCmd
	// ResumeCmd resumes active operation
	ResumeCmd ResumeCmd
	// PlanResumeCmd resumes active operation
	PlanResumeCmd PlanResumeCmd
	// PlanCompleteCmd completes the operation plan
	PlanCompleteCmd PlanCompleteCmd
	// RollbackCmd performs operation rollback
	RollbackCmd RollbackCmd
	// UpdateCmd combines app update related commands
	UpdateCmd UpdateCmd
	// UpdateCheckCmd checks if a new app version is available
	UpdateCheckCmd UpdateCheckCmd
	// UpdateTriggerCmd launches app update
	UpdateTriggerCmd UpdateTriggerCmd
	// UpdateUploadCmd uploads new app version to local cluster
	UpdateUploadCmd UpdateUploadCmd
	// UpdateCompleteCmd marks update operation as complete
	UpdateCompleteCmd UpdateCompleteCmd
	// UpdateSystemCmd updates system packages
	UpdateSystemCmd UpdateSystemCmd
	// UpgradeCmd launches app upgrade
	UpgradeCmd UpgradeCmd
	// StatusCmd combines subcommands for displaying status information
	StatusCmd StatusCmd
	// StatusClusterCmd displays the current cluster status
	StatusClusterCmd StatusClusterCmd
	// StatusHistoryCmd displays the cluster status history
	StatusHistoryCmd StatusHistoryCmd
	// StatusResetCmd resets the cluster to active state
	StatusResetCmd StatusResetCmd
	// RegistryCmd allows to interact with the cluster private registry
	RegistryCmd RegistryCmd
	// RegistryListCmd displays images from the registry
	RegistryListCmd RegistryListCmd
	// BackupCmd launches app backup hook
	BackupCmd BackupCmd
	// RestoreCmd launches app restore hook
	RestoreCmd RestoreCmd
	// CheckCmd checks that the host satisfies app manifest requirements
	CheckCmd CheckCmd
	// AppCmd combines subcommands for app service
	AppCmd AppCmd
	// AppInstallCmd installs an application from an application image
	AppInstallCmd AppInstallCmd
	// AppListCmd shows all application releases
	AppListCmd AppListCmd
	// AppUpgradeCmd upgrades a release
	AppUpgradeCmd AppUpgradeCmd
	// AppRollbackCmd rolls back a release
	AppRollbackCmd AppRollbackCmd
	// AppUninstallCmd uninstalls a release
	AppUninstallCmd AppUninstallCmd
	// AppHistoryCmd displays revision history for a release
	AppHistoryCmd AppHistoryCmd
	// AppSyncCmd synchronizes an application image with a cluster
	AppSyncCmd AppSyncCmd
	// AppSearchCmd searches for applications.
	AppSearchCmd AppSearchCmd
	// AppRebuildIndexCmd rebuilds Helm chart repository index.
	AppRebuildIndexCmd AppRebuildIndexCmd
	// AppIndexCmd generates Helm chart repository index file.
	AppIndexCmd AppIndexCmd
	// AppImportCmd imports an app into cluster
	AppImportCmd AppImportCmd
	// AppExportCmd exports specified app into registry
	AppExportCmd AppExportCmd
	// AppDeleteCmd deletes the specified app
	AppDeleteCmd AppDeleteCmd
	// AppPackageListCmd lists all app packages
	AppPackageListCmd AppPackageListCmd
	// AppPackageUninstallCmd launches app uninstall hook
	AppPackageUninstallCmd AppPackageUninstallCmd
	// AppStatusCmd output app status
	AppStatusCmd AppStatusCmd
	// AppPullCmd pulls app from specified cluster
	AppPullCmd AppPullCmd
	// AppPushCmd pushes app to specified cluster
	AppPushCmd AppPushCmd
	// AppHookCmd launches specified app hook
	AppHookCmd AppHookCmd
	// AppUnpackCmd unpacks specified app resources
	AppUnpackCmd AppUnpackCmd
	// WizardCmd starts installer in UI mode
	WizardCmd WizardCmd
	// AppPackageCmd displays the name of app in installer tarball
	AppPackageCmd AppPackageCmd
	// OpsCmd combines subcommands for ops service
	OpsCmd OpsCmd
	// OpsConnectCmd logs into specified cluster
	OpsConnectCmd OpsConnectCmd
	// OpsDisconnectCmd logs out of specified cluster
	OpsDisconnectCmd OpsDisconnectCmd
	// OpsListCmd lists ops credentials
	OpsListCmd OpsListCmd
	// OpsAgentCmd launches install agent
	OpsAgentCmd OpsAgentCmd
	// PackCmd combines subcommands for package service
	PackCmd PackCmd
	// PackImportCmd imports package into cluster
	PackImportCmd PackImportCmd
	// PackUnpackCmd unpacks specified package
	PackUnpackCmd PackUnpackCmd
	// PackExportCmd exports package from cluster
	PackExportCmd PackExportCmd
	// PackListCmd lists packages
	PackListCmd PackListCmd
	// PackDeleteCmd deletes specified package
	PackDeleteCmd PackDeleteCmd
	// PackConfigureCmd configures package
	PackConfigureCmd PackConfigureCmd
	// PackCommandCmd launches package command
	PackCommandCmd PackCommandCmd
	// PackPushCmd pushes package into specified cluster
	PackPushCmd PackPushCmd
	// PackPullCmd pulls package from specified cluster
	PackPullCmd PackPullCmd
	// PackLabelsCmd updates package labels
	PackLabelsCmd PackLabelsCmd
	// UserCmd combines user related subcommands
	UserCmd UserCmd
	// UserCreateCmd creates a new user
	UserCreateCmd UserCreateCmd
	// UserDeleteCmd deletes specified user
	UserDeleteCmd UserDeleteCmd
	// UsersCmd combines users related subcommands
	UsersCmd UsersCmd
	// UsersInviteCmd generates a new user invite link
	UsersInviteCmd UsersInviteCmd
	// UsersResetCmd generates a user password reset link
	UsersResetCmd UsersResetCmd
	// APIKeyCmd combines subcommands for API tokens
	APIKeyCmd APIKeyCmd
	// APIKeyCreateCmd creates a new token
	APIKeyCreateCmd APIKeyCreateCmd
	// APIKeyListCmd lists tokens
	APIKeyListCmd APIKeyListCmd
	// APIKeyDeleteCmd deletes specified token
	APIKeyDeleteCmd APIKeyDeleteCmd
	// ReportCmd generates cluster debug report
	ReportCmd ReportCmd
	// SiteCmd combines cluster related subcommands
	SiteCmd SiteCmd
	// SiteListCmd lists all clusters
	SiteListCmd SiteListCmd
	// SiteStartCmd starts gravity site
	SiteStartCmd SiteStartCmd
	// SiteInitCmd initializes gravity site from specified state
	SiteInitCmd SiteInitCmd
	// SiteStatusCmd displays cluster status
	SiteStatusCmd SiteStatusCmd
	// SiteInfoCmd displays some cluster information
	SiteInfoCmd SiteInfoCmd
	// SiteCompleteCmd marks cluster as finished final install step
	SiteCompleteCmd SiteCompleteCmd
	// SiteResetPasswordCmd resets password for local cluster user
	SiteResetPasswordCmd SiteResetPasswordCmd
	// LocalSiteCmd displays local cluster name
	LocalSiteCmd LocalSiteCmd
	// RPCAgentCmd combines subcommands for RPC agents
	RPCAgentCmd RPCAgentCmd
	// RPCAgentDeployCmd deploys RPC agents on cluster nodes
	RPCAgentDeployCmd RPCAgentDeployCmd
	// RPCAgentShutdownCmd requests RPC agents to shut down
	RPCAgentShutdownCmd RPCAgentShutdownCmd
	// RPCAgentInstallCmd installs and launches local RPC agent service
	RPCAgentInstallCmd RPCAgentInstallCmd
	// RPCAgentRunCmd runs RPC agent
	RPCAgentRunCmd RPCAgentRunCmd
	// RPCAgentStatusCmd requests RPC agent statuses
	RPCAgentStatusCmd RPCAgentStatusCmd
	// SystemCmd combines system subcommands
	SystemCmd SystemCmd
	// SystemTeleportCmd combines internal Teleport commands
	SystemTeleportCmd SystemTeleportCmd
	// SystemTeleportShowConfigCmd displays Teleport config
	SystemTeleportShowConfigCmd SystemTeleportShowConfigCmd
	// SystemRotateCertsCmd renews cluster certificates on local node
	SystemRotateCertsCmd SystemRotateCertsCmd
	// SystemExportCACmd exports cluster CA
	SystemExportCACmd SystemExportCACmd
	// SystemUninstallCmd uninstalls all gravity services from local node
	SystemUninstallCmd SystemUninstallCmd
	// SystemPullUpdatesCmd pulls updates for system packages
	SystemPullUpdatesCmd SystemPullUpdatesCmd
	// SystemUpdateCmd updates system packages
	SystemUpdateCmd SystemUpdateCmd
	// SystemReinstallCmd reinstalls specified system package
	SystemReinstallCmd SystemReinstallCmd
	// SystemHistoryCmd displays system update history
	SystemHistoryCmd SystemHistoryCmd
	// SystemClusterInfoCmd dumps cluster info suitable for debugging
	SystemClusterInfoCmd SystemClusterInfoCmd
	// SystemStepDownCmd asks active gravity master to step down
	SystemStepDownCmd SystemStepDownCmd
	// SystemRollbackCmd rolls back last system update
	SystemRollbackCmd SystemRollbackCmd
	// SystemServiceCmd combines subcommands for systems services
	SystemServiceCmd SystemServiceCmd
	// SystemServiceInstallCmd installs systemd service
	SystemServiceInstallCmd SystemServiceInstallCmd
	// SystemServiceUninstallCmd uninstalls systemd service
	SystemServiceUninstallCmd SystemServiceUninstallCmd
	// SystemServiceStatusCmd queries the runtime status of a package service
	SystemServiceStatusCmd SystemServiceStatusCmd
	// SystemServiceListCmd lists systemd services
	SystemServiceListCmd SystemServiceListCmd
	// SystemServiceStopCmd stops a package service
	SystemServiceStopCmd SystemServiceStopCmd
	// SystemServiceStartCmd stops or restarts a package service
	SystemServiceStartCmd SystemServiceStartCmd
	// SystemServiceJournalCmd queries the system journal of a package service
	SystemServiceJournalCmd SystemServiceJournalCmd
	// SystemReportCmd generates tarball with system diagnostics information
	SystemReportCmd SystemReportCmd
	// SystemStateDirCmd shows local state directory
	SystemStateDirCmd SystemStateDirCmd
	// SystemExportRuntimeJournalCmd exports runtime journal to a file
	SystemExportRuntimeJournalCmd SystemExportRuntimeJournalCmd
	// SystemStreamRuntimeJournalCmd streams contents of the runtime journal to a file
	SystemStreamRuntimeJournalCmd SystemStreamRuntimeJournalCmd
	// SystemSelinuxBootstrapCmd configures SELinux file contexts and ports on the node
	SystemSelinuxBootstrapCmd SystemSelinuxBootstrapCmd
	// SystemGCJournalCmd cleans up stale journal files
	SystemGCJournalCmd SystemGCJournalCmd
	// SystemGCPackageCmd removes unused packages
	SystemGCPackageCmd SystemGCPackageCmd
	// SystemGCRegistryCmd removes unused docker images
	SystemGCRegistryCmd SystemGCRegistryCmd
	// SystemEtcdCmd manipulates etcd cluster
	SystemEtcdCmd SystemEtcdCmd
	// SystemEtcdMigrateCmd migrates etcd data directories between versions
	SystemEtcdMigrateCmd SystemEtcdMigrateCmd
	// GarbageCollectCmd prunes unused resources (package/journal files/docker images)
	// in the cluster
	GarbageCollectCmd GarbageCollectCmd
	// PlanetCmd combines planet subcommands
	PlanetCmd PlanetCmd
	// [DEPRECATED] PlanetEnterCmd enters planet container
	PlanetEnterCmd PlanetEnterCmd
	// ExecCmd executes a command in a running container
	ExecCmd ExecCmd
	// ShellCmd starts interactive shell in a running planet container
	ShellCmd ShellCmd
	// PlanetStatusCmd displays planet status
	PlanetStatusCmd PlanetStatusCmd
	// EnterCmd enters planet container
	EnterCmd EnterCmd
	// ResourceCmd combines resource related subcommands
	ResourceCmd ResourceCmd
	// ResourceCreateCmd creates specified resource
	ResourceCreateCmd ResourceCreateCmd
	// ResourceRemoveCmd removes specified resource
	ResourceRemoveCmd ResourceRemoveCmd
	// ResourceGetCmd shows specified resource
	ResourceGetCmd ResourceGetCmd
	// TopCmd displays cluster metrics in terminal
	TopCmd TopCmd
}

// VersionCmd displays the binary version
type VersionCmd struct {
	*kingpin.CmdClause
	// Output is output format
	Output *constants.Format
}

// DNSConfig returns DNS configuration
func (r InstallCmd) DNSConfig() (config storage.DNSConfig) {
	for _, addr := range *r.DNSListenAddrs {
		config.Addrs = append(config.Addrs, addr.String())
	}
	config.Port = *r.DNSPort
	return config
}

// InstallCmd launches cluster installation
type InstallCmd struct {
	*kingpin.CmdClause
	// Path is the state directory path
	Path *string
	// AdvertiseAddr is local node advertise IP address
	AdvertiseAddr *string
	// Token is unique install token
	Token *string
	// CloudProvider enables cloud provider integration
	CloudProvider *string
	// Cluster is cluster name
	Cluster *string
	// App is app to install
	App *string
	// Flavor is app flavor to install
	Flavor *string
	// Role is local node profile
	Role *string
	// ResourcesPath is the path to user defined Kubernetes resources
	ResourcesPath *string
	// Wizard launches UI installer mode
	Wizard *bool
	// Mode is installation mode
	Mode *string
	// SystemDevice is device to use for system data
	SystemDevice *string
	// Mounts is a list of additional app mounts
	Mounts *configure.KeyVal
	// PodCIDR overrides default pod network
	PodCIDR *string
	// ServiceCIDR overrides default service network
	ServiceCIDR *string
	// VxlanPort overrides default overlay network port
	VxlanPort *int
	// DNSListenAddrs specifies listen addresses for planet DNS.
	DNSListenAddrs *[]net.IP
	// DNSPort overrides default DNS port for planet DNS.
	DNSPort *int
	// DockerStorageDriver specifies Docker storage driver to use
	DockerStorageDriver *dockerStorageDriver
	// DockerArgs specifies additional Docker arguments
	DockerArgs *[]string
	// ServiceUID is system user ID
	ServiceUID *string
	// ServiceGID is system user group ID
	ServiceGID *string
	// GCENodeTags lists additional node tags on GCE
	GCENodeTags *[]string
	// DNSHosts is a list of DNS host overrides
	DNSHosts *[]string
	// DNSZones is a list of DNS zone overrides
	DNSZones *[]string
	// Remote specifies whether the host should not be part of the cluster
	Remote *bool
	// SELinux specifies whether to run with SELinux support.
	// This flag makes the installer run in its own SELinux domain
	SELinux *bool
	// FromService specifies whether this process runs in service mode.
	//
	// The installer runs the main installer code in service mode, while
	// the client will simply connect to the service and stream its output and errors
	// and control whether it should stop
	FromService *bool
	// Set is a list of Helm chart values set on the CLI.
	Set *[]string
	// Values is a list of YAML files with Helm chart values.
	Values *[]string
	// AcceptEULA allows to auto-accept end-user license agreement.
	AcceptEULA *bool
}

// JoinCmd joins to the installer or existing cluster
type JoinCmd struct {
	*kingpin.CmdClause
	// PeerAddr is installer or cluster address
	PeerAddr *string
	// AdvertiseAddr is local node advertise IP address
	AdvertiseAddr *string
	// Token is join token
	Token *string
	// Role is local node profile
	Role *string
	// SystemDevice is device to use for system data
	SystemDevice *string
	// ServerAddr is RPC server address
	ServerAddr *string
	// Mounts is additional app mounts
	Mounts *configure.KeyVal
	// CloudProvider turns on cloud provider integration
	CloudProvider *string
	// OperationID is the ID of the operation created via UI
	OperationID *string
	// SELinux specifies whether to run with SELinux support.
	// This flag makes the installer run in its own SELinux domain
	SELinux *bool
	// FromService specifies whether this process runs in service mode.
	//
	// The agent runs the install/join code in service mode, while
	// the client will simply connect to the service and stream its output and errors
	// and control whether it should stop
	FromService *bool
	// StateDir is the operation-specific local state directory path
	StateDir *string
}

// AutoJoinCmd uses cloud provider info to join existing cluster
type AutoJoinCmd struct {
	*kingpin.CmdClause
	// ClusterName is cluster name
	ClusterName *string
	// Role is new node profile
	Role *string
	// SystemDevice is device to use for system data
	SystemDevice *string
	// Mounts is additional app mounts
	Mounts *configure.KeyVal
	// ServiceAddr specifies the service URL of the cluster to join
	ServiceAddr *string
	// AdvertiseAddr is local node advertise IP address
	AdvertiseAddr *string
	// Token is join token
	Token *string
	// SELinux specifies whether to run with SELinux support.
	// This flag makes the installer run in its own SELinux domain
	SELinux *bool
	// FromService specifies whether this process runs in service mode.
	//
	// The agent runs the install/join code in service mode, while
	// the client will simply connect to the service and stream its output and errors
	// and control whether it should stop
	FromService *bool
	// StateDir is the operation-specific local state directory path
	StateDir *string
}

// LeaveCmd removes the current node from the cluster
type LeaveCmd struct {
	*kingpin.CmdClause
	// Force suppresses operation failures
	Force *bool
	// Confirm suppresses confirmation prompt
	Confirm *bool
}

// RemoveCmd removes the specified node from the cluster
type RemoveCmd struct {
	*kingpin.CmdClause
	// Node is the node to remove
	Node *string
	// Force suppresses operation failures
	Force *bool
	// Confirm suppresses confirmation prompt
	Confirm *bool
}

// ResumeCmd resumes active operation
type ResumeCmd struct {
	*kingpin.CmdClause
	// OperationID is optional ID of operation to show the plan for
	OperationID *string
	// SkipVersionCheck suppresses version mismatch errors
	SkipVersionCheck *bool
	// Force forces rollback of the phase given in Phase
	Force *bool
	// PhaseTimeout is the rollback timeout
	PhaseTimeout *time.Duration
}

// PlanCmd manages an operation plan
type PlanCmd struct {
	*kingpin.CmdClause
	// OperationID is optional ID of operation to show the plan for
	OperationID *string
	// SkipVersionCheck suppresses version mismatch errors
	SkipVersionCheck *bool
}

// PlanDisplayCmd displays plan of a specific operation
type PlanDisplayCmd struct {
	*kingpin.CmdClause
	// Output is output format
	Output *constants.Format
	// Short is a shorthand for short output format
	Short *bool
	// Follow allows to follow the operation plan progress
	Follow *bool
}

// PlanExecuteCmd executes a phase of an active operation
type PlanExecuteCmd struct {
	*kingpin.CmdClause
	// Phase is the phase to execute
	Phase *string
	// Force forces execution of the given phase
	Force *bool
	// PhaseTimeout is the execution timeout
	PhaseTimeout *time.Duration
}

// PlanRollbackCmd rolls back a phase of an active operation
type PlanRollbackCmd struct {
	*kingpin.CmdClause
	// Phase is the phase to rollback
	Phase *string
	// Force forces rollback of the phase given in Phase
	Force *bool
	// PhaseTimeout is the rollback timeout
	PhaseTimeout *time.Duration
}

// PlanSetCmd sets the specified phase state without executing it
type PlanSetCmd struct {
	*kingpin.CmdClause
	// Phase is the phase to set state for
	Phase *string
	// State is the new phase state
	State *string
}

// PlanResumeCmd resumes active operation
type PlanResumeCmd struct {
	*kingpin.CmdClause
	// Force forces rollback of the phase given in Phase
	Force *bool
	// PhaseTimeout is the rollback timeout
	PhaseTimeout *time.Duration
	// Block indicates whether the command should run in foreground or as a systemd unit
	Block *bool
}

// PlanCompleteCmd completes the operation plan
type PlanCompleteCmd struct {
	*kingpin.CmdClause
}

// RollbackCmd performs operation rollback
type RollbackCmd struct {
	*kingpin.CmdClause
	// PhaseTimeout is the individual phase rollback timeout
	PhaseTimeout *time.Duration
	// OperationID is optional ID of operation to rollback
	OperationID *string
	// SkipVersionCheck suppresses version mismatch errors
	SkipVersionCheck *bool
	// Confirmed suppresses confirmation prompt
	Confirmed *bool
	// DryRun prints rollback phases without actually performing them
	DryRun *bool
}

// InstallPlanCmd combines subcommands for install plan
type InstallPlanCmd struct {
	*kingpin.CmdClause
}

// InstallPlanDisplayCmd displays install operation plan
type InstallPlanDisplayCmd struct {
	*kingpin.CmdClause
	// Output is output format
	Output *constants.Format
}

// UpgradePlanCmd combines subcommands for upgrade plan
type UpgradePlanCmd struct {
	*kingpin.CmdClause
}

// UpgradePlanDisplayCmd displays upgrade operation plan
type UpgradePlanDisplayCmd struct {
	*kingpin.CmdClause
	// Output is output format
	Output *constants.Format
}

// UpdateCmd combines update related subcommands
type UpdateCmd struct {
	*kingpin.CmdClause
}

// UpdateCheckCmd checks if a new app version is available
type UpdateCheckCmd struct {
	*kingpin.CmdClause
	// App is app name
	App *string
}

// UpdateTriggerCmd launches app update
type UpdateTriggerCmd struct {
	*kingpin.CmdClause
	// App is app name
	App *string
	// Manual starts operation in manual mode
	Manual *bool
	// SkipVersionCheck suppresses version mismatch errors
	SkipVersionCheck *bool
	// Force forces update
	Force *bool
}

// UpdateUploadCmd uploads new app version to local cluster
type UpdateUploadCmd struct {
	*kingpin.CmdClause
	// OpsCenterURL is cluster URL
	OpsCenterURL *string
}

// UpdateCompleteCmd marks update operation as completed
type UpdateCompleteCmd struct {
	*kingpin.CmdClause
	// Failed marks operation as failed
	Failed *bool
}

// UpdateSystemCmd updates system packages
type UpdateSystemCmd struct {
	*kingpin.CmdClause
	// ChangesetID is current changeset ID
	ChangesetID *string
	// ServiceName is systemd service name to launch
	ServiceName *string
	// WithStatus is whether to wait for service to complete
	WithStatus *bool
	// RuntimePackage specifies the runtime package to update to
	RuntimePackage *loc.Locator
}

// UpdatePlanInitCmd creates a new update operation plan
type UpdatePlanInitCmd struct {
	*kingpin.CmdClause
}

// UpgradeCmd launches app upgrade
type UpgradeCmd struct {
	*kingpin.CmdClause
	// App is app name
	App *string
	// Manual starts upgrade in manual mode
	Manual *bool
	// Phase is upgrade operation phase to execute
	Phase *string
	// Timeout is phase execution timeout
	Timeout *time.Duration
	// Force forces phase execution
	Force *bool
	// Resume resumes failed upgrade
	Resume *bool
	// SkipVersionCheck suppresses version mismatch errors
	SkipVersionCheck *bool
	// Set is a list of Helm chart values set on the CLI.
	Set *[]string
	// Values is a list of YAML files with Helm chart values.
	Values *[]string
	// Block indicates whether the command should run in foreground or as a systemd unit
	Block *bool
}

// StatusCmd combines subcommands for displaying status information
type StatusCmd struct {
	*kingpin.CmdClause
}

// StatusClusterCmd displays current cluster status
type StatusClusterCmd struct {
	*kingpin.CmdClause
	// Token displays only join token
	Token *bool
	// Tail follows current operation logs
	Tail *bool
	// OperationID displays operation status
	OperationID *string
	// Seconds displays status continuously
	Seconds *int
	// Output is output format
	Output *constants.Format
}

// StatusHistoryCmd displays cluster status history
type StatusHistoryCmd struct {
	*kingpin.CmdClause
}

// StatusResetCmd resets cluster to active state
type StatusResetCmd struct {
	*kingpin.CmdClause
	// Confirmed suppresses confirmation prompt
	Confirmed *bool
}

// RegistryCmd allows to interact with the cluster private registry
type RegistryCmd struct {
	*kingpin.CmdClause
}

// RegistryListCmd lists images in the registry
type RegistryListCmd struct {
	*kingpin.CmdClause
	// Registry is the address of registry to list contents in
	Registry *string
	// CAPath is path to registry CA certificate
	CAPath *string
	// CertPath is path to registry client certificate
	CertPath *string
	// KeyPath is path to registry client private key
	KeyPath *string
	// Format is the output format
	Format *constants.Format
}

// BackupCmd launches app backup hook
type BackupCmd struct {
	*kingpin.CmdClause
	// Tarball is backup tarball name
	Tarball *string
	// Timeout is operation timeout
	Timeout *time.Duration
	// Follow tails operation logs
	Follow *bool
}

// RestoreCmd launches app restore hook
type RestoreCmd struct {
	*kingpin.CmdClause
	// Tarball is tarball to restore from
	Tarball *string
	// Timeout is operation timeout
	Timeout *time.Duration
	// Follow tails operation logs
	Follow *bool
}

// CheckCmd checks that the host satisfies app manifest requirements
type CheckCmd struct {
	*kingpin.CmdClause
	// ManifestFile is path to app manifest file
	ManifestFile *string
	// Profile is profile name to check against
	Profile *string
	// AutoFix enables automatic fixing of some failed checks
	AutoFix *bool
	// ImagePath is path to unpacked cluster image
	ImagePath *string
	// Timeout is the time allotted to run preflight checks
	Timeout *time.Duration
}

// AppCmd combines subcommands for app service
type AppCmd struct {
	*kingpin.CmdClause
	// TillerNamespace specifies namespace where Tiller server is running.
	TillerNamespace *string
}

// AppInstallCmd installs an application from an application image.
type AppInstallCmd struct {
	*kingpin.CmdClause
	// Image specifies the application image to install.
	Image *string
	// Name is an optional release name.
	Name *string
	// Namespace is a namespace to install release into.
	Namespace *string
	// Set is a list of values set on the CLI.
	Set *[]string
	// Values is a list of YAML files with values.
	Values *[]string
	// Registry is a registry address where images will be pushed.
	Registry *string
	// RegistryCA is a registry CA certificate path.
	RegistryCA *string
	// RegistryCert is a registry client certificate path.
	RegistryCert *string
	// RegistryKey is a registry client private key path.
	RegistryKey *string
	// RegistryUsername is registry username for basic auth.
	RegistryUsername *string
	// RegistryPassword is registry password for basic auth.
	RegistryPassword *string
	// RegistryPrefix is registry prefix when pushing images.
	RegistryPrefix *string
}

// AppListCmd shows all application releases.
type AppListCmd struct {
	*kingpin.CmdClause
	// All displays releases with all possible statuses.
	All *bool
}

// AppUpgradeCmd upgrades a release.
type AppUpgradeCmd struct {
	*kingpin.CmdClause
	// Release is the release name to upgrade.
	Release *string
	// Image specifies the application image to upgrade to.
	Image *string
	// Set is a list of values set on the CLI.
	Set *[]string
	// Values is a list of YAML files with values.
	Values *[]string
	// Registry is a registry address where images will be pushed.
	Registry *string
	// RegistryCA is a registry CA certificate path.
	RegistryCA *string
	// RegistryCert is a registry client certificate path.
	RegistryCert *string
	// RegistryKey is a registry client private key path.
	RegistryKey *string
	// RegistryUsername is registry username for basic auth.
	RegistryUsername *string
	// RegistryPassword is registry password for basic auth.
	RegistryPassword *string
	// RegistryPrefix is registry prefix when pushing images.
	RegistryPrefix *string
}

// AppRollbackCmd rolls back a release.
type AppRollbackCmd struct {
	*kingpin.CmdClause
	// Release is a release to rollback.
	Release *string
	// Revision is a version number to rollback to.
	Revision *int
}

// AppUninstallCmd uninstalls a release.
type AppUninstallCmd struct {
	*kingpin.CmdClause
	// Release is a release name to uninstall.
	Release *string
}

// AppHistoryCmd displays application revision history.
type AppHistoryCmd struct {
	*kingpin.CmdClause
	// Release is a release name to display revisions for.
	Release *string
}

// AppSyncCmd synchronizes an application image with a cluster.
type AppSyncCmd struct {
	*kingpin.CmdClause
	// Image specifies the application image to sync.
	Image *string
	// Registry is a registry address where images will be pushed.
	Registry *string
	// RegistryCA is a registry CA certificate path.
	RegistryCA *string
	// RegistryCert is a registry client certificate path.
	RegistryCert *string
	// RegistryKey is a registry client private key path.
	RegistryKey *string
	// RegistryUsername is registry username for basic auth.
	RegistryUsername *string
	// RegistryPassword is registry password for basic auth.
	RegistryPassword *string
	// RegistryPrefix is registry prefix when pushing images.
	RegistryPrefix *string
	// ScanningRepository is a docker repository to push a copy of all vendored images
	// Used internally so the registry can scan those images and report on vulnerabilities
	ScanningRepository *string
	// ScanningTagPrefix is a prefix to add to each tag when pushed to help identify the image from the scan results
	ScanningTagPrefix *string
}

// AppSearchCmd searches for applications.
type AppSearchCmd struct {
	*kingpin.CmdClause
	// Pattern is an application name pattern.
	Pattern *string
	// Remote displays remote applications.
	Remote *bool
	// All displays both local and remote applications.
	All *bool
}

// AppRebuildIndexCmd rebuilds Helm chart repository index.
type AppRebuildIndexCmd struct {
	*kingpin.CmdClause
}

// AppIndexCmd generates Helm chart repository index file.
type AppIndexCmd struct {
	*kingpin.CmdClause
	// MergeInto is the index file to merge generated index file into.
	MergeInto *string
}

// AppImportCmd imports app into cluster
type AppImportCmd struct {
	*kingpin.CmdClause
	// Source is app tarball
	Source *string
	// Repository sets app repository
	Repository *string
	// Name sets app name
	Name *string
	// Version sets app version
	Version *string
	// RegistryURL is Docker registry URL
	RegistryURL *string
	// DockerURL is Docker daemon URL
	DockerURL *string
	// OpsCenterURL is cluster URL to import to
	OpsCenterURL *string
	// Vendor turns on Docker image vendoring
	Vendor *bool
	// Force overwrites existing app
	Force *bool
	// Excludes is a list of files to exclude from installer tarball
	Excludes *[]string
	// IncludePatterns is a list of files to include into tarball
	IncludePaths *[]string
	// VendorPatterns is a list of file patterns to exclude when searching for images
	VendorPatterns *[]string
	// VendorIgnorePatterns is a list of file patterns to look in for images
	VendorIgnorePatterns *[]string
	// SetImages sets specified image versions
	SetImages *loc.DockerImages
	// SetDeps sets specified dependency versions
	SetDeps *loc.Locators
	// Parallel defines the number of tasks to execute concurrently
	Parallel *int
}

// AppExportCmd exports specified app into registry
type AppExportCmd struct {
	*kingpin.CmdClause
	// Locator is app locator
	Locator *string
	// RegistryURL is Docker registry URL
	RegistryURL *string
	// OpsCenterURL is app service URL
	OpsCenterURL *string
}

// AppDeleteCmd deletes the specified app
type AppDeleteCmd struct {
	*kingpin.CmdClause
	// Locator is app locator
	Locator *string
	// OpsCenterURL is ops service URL
	OpsCenterURL *string
	// Force suppresses not found errors
	Force *bool
}

// AppPackageListCmd lists all app packages
type AppPackageListCmd struct {
	*kingpin.CmdClause
	// Repository is repository to list apps from
	Repository *string
	// Type is type of apps to list
	Type *string
	// ShowHidden shows hidden apps as well
	ShowHidden *bool
	// OpsCenterURL is app service URL
	OpsCenterURL *string
}

// AppPackageUninstallCmd launches application uninstall hook
type AppPackageUninstallCmd struct {
	*kingpin.CmdClause
	// Locator is the application locator
	Locator *loc.Locator
}

// AppStatusCmd shows app status
type AppStatusCmd struct {
	*kingpin.CmdClause
	// Locator is app locator
	Locator *loc.Locator
	// OpsCenterURL is app service URL
	OpsCenterURL *string
}

// AppPullCmd pulls app from specified cluster
type AppPullCmd struct {
	*kingpin.CmdClause
	// Package is app locator
	Package *loc.Locator
	// OpsCenterURL is app service URL to pull from
	OpsCenterURL *string
	// Labels is labels to apply to pulled app
	Labels *configure.KeyVal
	// Force overwrites existing app
	Force *bool
}

// AppPushCmd pushes app to specified cluster
type AppPushCmd struct {
	*kingpin.CmdClause
	// Package is app locator
	Package *loc.Locator
	// OpsCenterURL is app service URL to push to
	OpsCenterURL *string
}

// AppHookCmd launches specified app hook
type AppHookCmd struct {
	*kingpin.CmdClause
	// Package is app locator
	Package *loc.Locator
	// HookName specifies hook to launch
	HookName *string
	// Env is additional environment variables to provide to hook
	Env *map[string]string
}

// AppUnpackCmd unpacks app resources
type AppUnpackCmd struct {
	*kingpin.CmdClause
	// Package is app locator
	Package *loc.Locator
	// Dir is unpack location
	Dir *string
	// OpsCenterURL is app service URL to pull app from
	OpsCenterURL *string
	// ServiceUID is user ID to change unpacked resources ownership to
	ServiceUID *string
}

// WizardCmd starts installer in UI mode
type WizardCmd struct {
	*kingpin.CmdClause
	// Path is the state directory path
	Path *string
	// ServiceUID is system user ID
	ServiceUID *string
	// ServiceGID is system user group ID
	ServiceGID *string
	// AdvertiseAddr specifies the advertise address for the wizard
	AdvertiseAddr *string
	// Token is unique install token
	Token *string
	// FromService specifies whether this process runs in service mode.
	//
	// The installer runs the main installer code in service mode, while
	// the client will simply connect to the service and stream its output and errors
	// and control whether it should stop
	FromService *bool
	// Set is a list of Helm chart values set on the CLI.
	Set *[]string
	// Values is a list of YAML files with Helm chart values.
	Values *[]string
}

// AppPackageCmd displays the name of app in installer tarball
type AppPackageCmd struct {
	*kingpin.CmdClause
}

// OpsCmd combines subcommands for ops service
type OpsCmd struct {
	*kingpin.CmdClause
}

// OpsConnectCmd logs into specified cluster
type OpsConnectCmd struct {
	*kingpin.CmdClause
	// OpsCenterURL is ops service URL
	OpsCenterURL *string
	// Username is agent username
	Username *string
	// Password is agent password
	Password *string
}

// OpsDisconnectCmd logs out of specified cluster
type OpsDisconnectCmd struct {
	*kingpin.CmdClause
	// OpsCenterURL is ops service URL
	OpsCenterURL *string
}

// OpsListCmd lists ops credentials
type OpsListCmd struct {
	*kingpin.CmdClause
}

// OpsAgentCmd launches install agent
type OpsAgentCmd struct {
	*kingpin.CmdClause
	// PackageAddr is address of package service to bootstrap credentials from
	PackageAddr *string
	// AdvertiseAddr is agent advertise IP address
	AdvertiseAddr *net.IP
	// ServerAddr is RPC server address
	ServerAddr *string
	// Token is agent token
	Token *string
	// ServiceName is systemd service name to launch
	ServiceName *string
	// Vars is additional agent vars
	Vars *configure.KeyVal
	// ServiceUID is system user ID
	ServiceUID *string
	// ServiceGID is system user group ID
	ServiceGID *string
	// CloudProvider enables cloud provider integration
	CloudProvider *string
}

// PackCmd combines subcommands for package service
type PackCmd struct {
	*kingpin.CmdClause
}

// PackImportCmd imports package into cluster
type PackImportCmd struct {
	*kingpin.CmdClause
	// CheckManifest validates package manifest
	CheckManifest *bool
	// OpsCenterURL is pack service URL
	OpsCenterURL *string
	// Path is package tarball
	Path *string
	// Locator is package locator
	Locator *loc.Locator
	// Labels is labels to update pulled package with
	Labels *configure.KeyVal
}

// PackUnpackCmd unpacks specified package
type PackUnpackCmd struct {
	*kingpin.CmdClause
	// Locator is package locator
	Locator *loc.Locator
	// Dir is directory to unpack to
	Dir *string
	// OpsCenterURL is pack service URL
	OpsCenterURL *string
}

// PackExportCmd exports package from cluster
type PackExportCmd struct {
	*kingpin.CmdClause
	// Locator is package locator
	Locator *loc.Locator
	// File is file name to export to
	File *string
	// OpsCenterURL is pack service URL
	OpsCenterURL *string
	// FileMask is file mask for exported package
	FileMask *string
	// FileLabel optionally specifies SELinux label
	FileLabel *string
}

// PackListCmd lists packages
type PackListCmd struct {
	*kingpin.CmdClause
	// Repository is repository to list packages from
	Repository *string
	// OpsCenterURL is pack service URL
	OpsCenterURL *string
}

// PackDeleteCmd deletes specified package
type PackDeleteCmd struct {
	*kingpin.CmdClause
	// Force suppresses not found errors
	Force *bool
	// Locator is package locator
	Locator *loc.Locator
	// OpsCenterURL is pack service URL
	OpsCenterURL *string
}

// PackConfigureCmd configures package
type PackConfigureCmd struct {
	*kingpin.CmdClause
	// Package is package locator
	Package *loc.Locator
	// ConfPackage is configuration package locator
	ConfPackage *loc.Locator
	// Args is additional arguments to the configure command
	Args *[]string
}

// PackCommandCmd launches package command
type PackCommandCmd struct {
	*kingpin.CmdClause
	// Command is package command to run
	Command *string
	// Package is package locator
	Package *loc.Locator
	// ConfPackage is configuration package locator
	ConfPackage *loc.Locator
	// Args is additional arguments to the package command
	Args *[]string
}

// PackPushCmd pushes package into specified cluster
type PackPushCmd struct {
	*kingpin.CmdClause
	// Package is package locator
	Package *loc.Locator
	// OpsCenterURL is pack service URL to push into
	OpsCenterURL *string
}

// PackPullCmd pulls package from specified cluster
type PackPullCmd struct {
	*kingpin.CmdClause
	// Package is package locator
	Package *loc.Locator
	// OpsCenterURL is pack service URL to pull from
	OpsCenterURL *string
	// Labels is labels to update pulled package with
	Labels *configure.KeyVal
	// Force overwrites existing package
	Force *bool
}

// PackLabelsCmd updates package labels
type PackLabelsCmd struct {
	*kingpin.CmdClause
	// Package is package name
	Package *loc.Locator
	// OpsCenterURL is pack service URL
	OpsCenterURL *string
	// Add is a map of labels to add
	Add *configure.KeyVal
	// Remove is a list of labels to remove
	Remove *[]string
}

// UserCmd combines user related subcommands
type UserCmd struct {
	*kingpin.CmdClause
}

// UserCreateCmd creates a new user
type UserCreateCmd struct {
	*kingpin.CmdClause
	// Email is user email
	Email *string
	// Type is user type
	Type *string
	// Password is user password
	Password *string
	// OpsCenterURL is users service URL
	OpsCenterURL *string
}

// UserDeleteCmd deletes specified user
type UserDeleteCmd struct {
	*kingpin.CmdClause
	// Email is user email
	Email *string
	// OpsCenterURL is users service URL
	OpsCenterURL *string
}

// UsersCmd combines user related subcommands
type UsersCmd struct {
	*kingpin.CmdClause
}

// UsersInviteCmd generates a new user invite link
type UsersInviteCmd struct {
	*kingpin.CmdClause
	// Name is user name
	Name *string
	// Roles is user roles
	Roles *[]string
	// TTL is invite link TTL
	TTL *time.Duration
}

// UserResetCmd generates a user password reset link
type UsersResetCmd struct {
	*kingpin.CmdClause
	// Name is user name
	Name *string
	// TTL is reset link TTL
	TTL *time.Duration
}

// APIKeyCmd combines subcommands for API tokens
type APIKeyCmd struct {
	*kingpin.CmdClause
}

// APIKeyCreateCmd creates a new token
type APIKeyCreateCmd struct {
	*kingpin.CmdClause
	// Email is user token is for
	Email *string
	// OpsCenterURL is cluster URL
	OpsCenterURL *string
}

// APIKeyListCmd lists tokens
type APIKeyListCmd struct {
	*kingpin.CmdClause
	// Email is user to show tokens for
	Email *string
	// OpsCenterURL is cluster URL
	OpsCenterURL *string
}

// APIKeyDeleteCmd deletes specified token
type APIKeyDeleteCmd struct {
	*kingpin.CmdClause
	// Token is token to delete
	Token *string
	// Email is user the token belongs to
	Email *string
	// OpsCenterURL is cluster URL
	OpsCenterURL *string
}

// ReportCmd generates cluster debug report
type ReportCmd struct {
	*kingpin.CmdClause
	// FilePath is the report tarball path
	FilePath *string
	// Since is the duration before now that specifies the start of the time
	// filter. Only log entries from the start of the time filter until now will
	// be included in the report.
	Since *time.Duration
}

// SiteCmd combines cluster related subcommands
type SiteCmd struct {
	*kingpin.CmdClause
}

// SiteListCmd lists all clusters
type SiteListCmd struct {
	*kingpin.CmdClause
	// OpsCenterURL is cluster URL
	OpsCenterURL *string
}

// SiteStartCmd starts gravity site
type SiteStartCmd struct {
	*kingpin.CmdClause
	// ConfigPath is path to config file
	ConfigPath *string
	// InitPath is path to init from
	InitPath *string
}

// SiteInitCmd initializes gravity site from specified state
type SiteInitCmd struct {
	*kingpin.CmdClause
	// ConfigPath is path to config file
	ConfigPath *string
	// InitPath is path to init from
	InitPath *string
}

// SiteStatusCmd displays cluster status
type SiteStatusCmd struct {
	*kingpin.CmdClause
}

// SiteInfoCmd displays some cluster information
type SiteInfoCmd struct {
	*kingpin.CmdClause
	// DomainName is cluster name
	DomainName *string
	// Format is output format
	Format *constants.Format
}

// SiteCompleteCmd marks cluster as finished final install step
type SiteCompleteCmd struct {
	*kingpin.CmdClause
	// Support turns on remote support
	Support *string
}

// SiteResetPasswordCmd resets password for local cluster user
type SiteResetPasswordCmd struct {
	*kingpin.CmdClause
}

// LocalSiteCmd displays local cluster name
type LocalSiteCmd struct {
	*kingpin.CmdClause
}

// RPCAgentCmd combines subcommands for RPC agents
type RPCAgentCmd struct {
	*kingpin.CmdClause
}

// RPCAgentDeployCmd deploys RPC agents on cluster nodes
type RPCAgentDeployCmd struct {
	*kingpin.CmdClause
	// LeaderArgs is additional arguments to the leader agent
	LeaderArgs *string
	// NodeArgs is additional arguments to the regular agent
	NodeArgs *string
	// Version specifies the version of the agent to be deployed
	Version *string
}

// RPCAgentShutdownCmd requests RPC agents to shut down
type RPCAgentShutdownCmd struct {
	*kingpin.CmdClause
}

// RPCAgentInstallCmd installs and launches local RPC agent service
type RPCAgentInstallCmd struct {
	*kingpin.CmdClause
	// Args is additional arguments to the agent
	Args *[]string
}

// RPCAgentRunCmd runs RPC agent
type RPCAgentRunCmd struct {
	*kingpin.CmdClause
	// Args is additional arguments to the agent
	Args *[]string
}

// RPCAgentStatusCmd requests RPC agent statuses
type RPCAgentStatusCmd struct {
	*kingpin.CmdClause
}

// SystemCmd combines system subcommands
type SystemCmd struct {
	*kingpin.CmdClause
}

// SystemTeleportCmd combines internal Teleport commands
type SystemTeleportCmd struct {
	*kingpin.CmdClause
}

// SystemTeleportShowConfigCmd displays Teleport config from specified package
type SystemTeleportShowConfigCmd struct {
	*kingpin.CmdClause
	// Package is the package to show config from
	Package *string
}

// SystemRotateCertsCmd renews cluster certificates on local node
type SystemRotateCertsCmd struct {
	*kingpin.CmdClause
	// ClusterName is local cluster name
	ClusterName *string
	// ValidFor is validity period for new certificates
	ValidFor *time.Duration
	// CAPath is CA to use
	CAPath *string
}

// SystemExportCACmd exports cluster CA
type SystemExportCACmd struct {
	*kingpin.CmdClause
	// ClusterName is local cluster name
	ClusterName *string
	// CAPath is path to export CA to
	CAPath *string
}

// StopCmd stops all Gravity services on the node.
type StopCmd struct {
	*kingpin.CmdClause
	// Confirmed suppresses confirmation prompt.
	Confirmed *bool
}

// StartCmd starts all Gravity services on the node.
type StartCmd struct {
	*kingpin.CmdClause
	// AdvertiseAddr is the new node advertise address.
	AdvertiseAddr *string
	// FromService indicates that the command is running as a systemd service.
	FromService *bool
	// StateDir is the local operation-specific state directory path
	StateDir *string
	// Confirmed suppresses confirmation prompt.
	Confirmed *bool
}

// SystemUninstallCmd uninstalls all gravity services from local node
type SystemUninstallCmd struct {
	*kingpin.CmdClause
	// Confirmed suppresses confirmation prompt
	Confirmed *bool
}

// SystemPullUpdatesCmd pulls updates for system packages
type SystemPullUpdatesCmd struct {
	*kingpin.CmdClause
	// OpsCenterURL is cluster URL
	OpsCenterURL *string
	// RuntimePackage specifies the runtime package to update to
	RuntimePackage *loc.Locator
}

// SystemUpdateCmd updates system packages
type SystemUpdateCmd struct {
	*kingpin.CmdClause
	// ChangesetID is changeset ID
	ChangesetID *string
	// ServiceName is systemd service name to launch
	ServiceName *string
	// WithStatus waits for operation to finish
	WithStatus *bool
	// RuntimePackage specifies the runtime package to update to
	RuntimePackage *loc.Locator
}

// SystemReinstallCmd reinstalls specified system package
type SystemReinstallCmd struct {
	*kingpin.CmdClause
	// Package is package locator
	Package *loc.Locator
	// ServiceName is systemd service name to launch
	ServiceName *string
	// Labels defines the labels to identify the package with
	Labels *configure.KeyVal
	// ClusterRole is the node's cluster role (master or node)
	ClusterRole *string
}

// SystemHistoryCmd displays system update history
type SystemHistoryCmd struct {
	*kingpin.CmdClause
}

// SystemClusterInfoCmd dumps kubernetes cluster info suitable for debugging.
// It is a convenience wrapper around 'kubectl cluster-info dump --all-namespaces'
type SystemClusterInfoCmd struct {
	*kingpin.CmdClause
}

// SystemStepDownCmd asks active gravity master to step down
type SystemStepDownCmd struct {
	*kingpin.CmdClause
}

// SystemRollbackCmd rolls back last system update
type SystemRollbackCmd struct {
	*kingpin.CmdClause
	// ChangesetID is changeset ID to rollback
	ChangesetID *string
	// ServiceName is systemd service name to launch
	ServiceName *string
	// WithStatus waits for operation to finish
	WithStatus *bool
}

// SystemServiceCmd combines subcommands for systems services
type SystemServiceCmd struct {
	*kingpin.CmdClause
}

// SystemServiceInstallCmd installs systemd service
type SystemServiceInstallCmd struct {
	*kingpin.CmdClause
	// Package is system service package locator
	Package *loc.Locator
	// ConfigPackage is config package locator
	ConfigPackage *loc.Locator
	// StartCommand is systemd unit StartCommand
	StartCommand *string
	// StartPreCommand is systemd unit StartPreCommand
	StartPreCommand *string
	// StartPostCommand is systemd unit StartPostCommand
	StartPostCommand *string
	// StopCommadn is systemd unit StopCommand
	StopCommand *string
	// StopPostCommand is systemd unit StopPostCommand
	StopPostCommand *string
	// Timeout is systemd unit timeout
	Timeout *int
	// Type is systemd unit type
	Type *string
	// Restart is systemd unit restart policy
	Restart *string
	// LimitNoFile is systemd unit file limit
	LimitNoFile *int
	// KillMode is systemd unit kill mode
	KillMode *string
}

// SystemServiceUninstallCmd uninstalls systemd service
type SystemServiceUninstallCmd struct {
	*kingpin.CmdClause
	// Package is system service package locator
	Package *loc.Locator
	// Name is service name
	Name *string
}

// SystemServiceStatusCmd queries the runtime status of a package service
type SystemServiceStatusCmd struct {
	*kingpin.CmdClause
	// Package specifies the service either a package locator
	// or a partial unique pattern (i.e. 'planet')
	Package *string
}

// SystemServiceListCmd lists systemd services
type SystemServiceListCmd struct {
	*kingpin.CmdClause
}

// SystemServiceStartCmd starts or restart a package service
type SystemServiceStartCmd struct {
	*kingpin.CmdClause
	// Package specifies the service either a package locator
	// or a partial unique pattern (i.e. 'planet')
	Package *string
}

// SystemServiceStopCmd stops a running package service
type SystemServiceStopCmd struct {
	*kingpin.CmdClause
	// Package specifies the service either a package locator
	// or a partial unique pattern (i.e. 'planet')
	Package *string
}

// SystemServiceJournalCmd queries the system journal of a package service
type SystemServiceJournalCmd struct {
	*kingpin.CmdClause
	// Package specifies the service either a package locator
	// or a partial unique pattern (i.e. 'planet')
	Package *string
	// Args optionally lists additional arguments to journalctl
	Args *[]string
}

// SystemReportCmd generates tarball with system diagnostics information
type SystemReportCmd struct {
	*kingpin.CmdClause
	// Filter allows to collect only specific diagnostics
	Filter *[]string
	// Compressed allows to gzip the tarball
	Compressed *bool
	// Output optionally specifies output file path
	Output *string
	// Since is the duration before now that specifies the start of the time
	// filter. Only log entries from the start of the time filter until now will
	// be included in the report.
	Since *time.Duration
}

// SystemStateDirCmd shows local state directory
type SystemStateDirCmd struct {
	*kingpin.CmdClause
}

// SystemExportRuntimeJournalCmd exports runtime journal to a file
type SystemExportRuntimeJournalCmd struct {
	*kingpin.CmdClause
	// OutputFile specifies the path of the resulting tarball
	OutputFile *string
	// Since is the duration before now that specifies the start of the time
	// filter. Only log entries from the start of the time filter until now will
	// be included in the report.
	Since *time.Duration
	// Export serializes the journal into a binary stream.
	Export *bool
}

// SystemStreamRuntimeJournalCmd streams contents of the runtime journal
type SystemStreamRuntimeJournalCmd struct {
	*kingpin.CmdClause
	// Since is the duration before now that specifies the start of the time
	// filter. Only log entries from the start of the time filter until now will
	// be included in the report.
	Since *time.Duration
	// Export serializes the journal into a binary stream.
	Export *bool
}

// SystemSelinuxBootstrapCmd configures SELinux file contexts and ports on the node
type SystemSelinuxBootstrapCmd struct {
	*kingpin.CmdClause
	// Path specifies the optional output file where the bootstrap script is saved.
	// In this case, the command does not execute the script
	Path *string
	// VxlanPort optionally specifies the new vxlan port
	VxlanPort *int
}

// SystemGCJournalCmd manages cleanup of journal files
type SystemGCJournalCmd struct {
	*kingpin.CmdClause
	// LogDir specifies the alternative location of the journal files.
	// If unspecified, defaults.JournalLogDir is used
	LogDir *string
	// MachineIDFile specifies the alternative location of the systemd machine-id file.
	// If unspecified, defaults.SystemdMachineID is used
	MachineIDFile *string
}

// SystemGCPackageCmd removes unused packages
type SystemGCPackageCmd struct {
	*kingpin.CmdClause
	// DryRun displays the packages to be removed
	// without actually removing anything
	DryRun *bool
	// Cluster specifies whether to prune cluster packages
	Cluster *bool
}

// SystemGCRegistryCmd removes unused docker images
type SystemGCRegistryCmd struct {
	*kingpin.CmdClause
	// Confirm specifies the user consent that external docker images
	// (images not part of the installation) are to be removed from the
	// local docker registry
	Confirm *bool
	// DryRun displays the images to be removed
	// without actually removing anything
	DryRun *bool
}

// SystemEtcdCmd manipulates etcd cluster
type SystemEtcdCmd struct {
	*kingpin.CmdClause
}

// SystemEtcdMigrateCmd migrates etcd data directories between versions
type SystemEtcdMigrateCmd struct {
	*kingpin.CmdClause
	// From specifies the source version, as a semver (without `v` prefix)
	From *string
	// To specifies the destination version, as a semver (without `v` prefix)
	To *string
}

// GarbageCollectCmd prunes unused cluster resources
type GarbageCollectCmd struct {
	*kingpin.CmdClause
	// Manual is whether the operation is not executed automatically
	Manual *bool
	// Confirmed is whether the user has confirmed the removal of custom docker
	// images
	Confirmed *bool
}

// GarbageCollectPlanCmd displays the plan of the garbage collection operation
type GarbageCollectPlanCmd struct {
	*kingpin.CmdClause
	// Format is the output format
	Format *constants.Format
}

// PlanetCmd combines planet subcommands
type PlanetCmd struct {
	*kingpin.CmdClause
}

// PlanetEnterCmd enters planet container
type PlanetEnterCmd struct {
	*kingpin.CmdClause
}

// PlanetStatusCmd displays planet status
type PlanetStatusCmd struct {
	*kingpin.CmdClause
}

// EnterCmd enters planet container
type EnterCmd struct {
	*kingpin.CmdClause
	// Args is additional arguments to the command
	Args *[]string
}

// ExecCmd runs process in the running container
type ExecCmd struct {
	*kingpin.CmdClause
	// TTY allocates a pseudo-TTY
	TTY *bool
	// Stdin attaches stdin
	Stdin *bool
	// Cmd is a command to execute
	Cmd *string
	// Args is additional arguments to the command Cmd
	Args *[]string
}

// ShellCmd is an alias for exec with -ti /bin/bash
type ShellCmd struct {
	*kingpin.CmdClause
}

// ResourceCmd combines resource related subcommands
type ResourceCmd struct {
	*kingpin.CmdClause
}

// ResourceCreateCmd creates specified resource
type ResourceCreateCmd struct {
	*kingpin.CmdClause
	// Filename is path to file with resource definition
	Filename *string
	// Upsert overwrites existing resource
	Upsert *bool
	// User is resource owner
	User *string
	// Manual controls whether an operation is created in manual mode.
	// If resource is managed with the help of a cluster operation,
	// setting this to true will not cause the operation to start automatically
	Manual *bool
	// Confirmed suppresses confirmation prompt
	Confirmed *bool
}

// ResourceRemoveCmd removes specified resource
type ResourceRemoveCmd struct {
	*kingpin.CmdClause
	// Kind is resource kind
	Kind *string
	// Name is resource name
	Name *string
	// Force suppresses not found errors
	Force *bool
	// User is resource owner
	User *string
	// Manual controls whether an operation is created in manual mode.
	// If resource is managed with the help of a cluster operation,
	// setting this to true will not cause the operation to start automatically
	Manual *bool
	// Confirmed suppresses confirmation prompt
	Confirmed *bool
}

// ResourceGetCmd shows specified resource
type ResourceGetCmd struct {
	*kingpin.CmdClause
	// Kind is resource kind
	Kind *string
	// Name is resource name
	Name *string
	// Format is output format
	Format *constants.Format
	// WithSecrets show normally hidden resource fields
	WithSecrets *bool
	// User is resource owner
	User *string
}

// TopCmd displays cluster metrics in terminal.
type TopCmd struct {
	*kingpin.CmdClause
	// Interval is the interval to display metrics for.
	Interval *time.Duration
	// Step is the max time b/w two datapoints.
	Step *time.Duration
}
