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
	// PlanCmd displays current operation plan
	PlanCmd PlanCmd
	// RollbackCmd rolls back the specified operation plan phase
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
	// StatusCmd displays cluster status
	StatusCmd StatusCmd
	// StatusResetCmd resets the cluster to active state
	StatusResetCmd StatusResetCmd
	// BackupCmd launches app backup hook
	BackupCmd BackupCmd
	// RestoreCmd launches app restore hook
	RestoreCmd RestoreCmd
	// CheckCmd checks that the host satisfies app manifest requirements
	CheckCmd CheckCmd
	// AppCmd combines subcommands for app service
	AppCmd AppCmd
	// AppImportCmd imports an app into cluster
	AppImportCmd AppImportCmd
	// AppExportCmd exports specified app into registry
	AppExportCmd AppExportCmd
	// AppDeleteCmd deletes the specified app
	AppDeleteCmd AppDeleteCmd
	// AppListCmd lists all apps
	AppListCmd AppListCmd
	// AppUninstallCmd launches app uninstall hook
	AppUninstallCmd AppUninstallCmd
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
	// SystemCmd combines system subcommands
	SystemCmd SystemCmd
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
	// SystemServiceStatusCmd displays status of systemd service
	SystemServiceStatusCmd SystemServiceStatusCmd
	// SystemServiceListCmd lists systemd services
	SystemServiceListCmd SystemServiceListCmd
	// SystemReportCmd generates tarball with system diagnostics information
	SystemReportCmd SystemReportCmd
	// SystemStateDirCmd shows local state directory
	SystemStateDirCmd SystemStateDirCmd
	// SystemDevicemapperCmd combines devicemapper related subcommands
	SystemDevicemapperCmd SystemDevicemapperCmd
	// SystemDevicemapperMountCmd configures devicemapper environment
	SystemDevicemapperMountCmd SystemDevicemapperMountCmd
	// SystemDevicemapperUnmountCmd removes devicemapper environment
	SystemDevicemapperUnmountCmd SystemDevicemapperUnmountCmd
	// SystemDevicemapperSystemDirCmd show LVM system directory
	SystemDevicemapperSystemDirCmd SystemDevicemapperSystemDirCmd
	// SystemEnablePromiscModeCmd puts network interface into promiscuous mode
	SystemEnablePromiscModeCmd SystemEnablePromiscModeCmd
	// SystemDisablePromiscModeCmd removes promiscuous mode from interface
	SystemDisablePromiscModeCmd SystemDisablePromiscModeCmd
	// SystemExportRuntimeJournalCmd exports runtime journal to a file
	SystemExportRuntimeJournalCmd SystemExportRuntimeJournalCmd
	// SystemStreamRuntimeJournalCmd streams contents of the runtime journal to a file
	SystemStreamRuntimeJournalCmd SystemStreamRuntimeJournalCmd
	// SystemGCJournalCmd cleans up stale journal files
	SystemGCJournalCmd SystemGCJournalCmd
	// SystemGCPackageCmd removes unused packages
	SystemGCPackageCmd SystemGCPackageCmd
	// SystemGCRegistryCmd removes unused docker images
	SystemGCRegistryCmd SystemGCRegistryCmd
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
	// DockerDevice is device to use for Docker data
	DockerDevice *string
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
	// Phase specifies the install phase ID to execute
	Phase *string
	// PhaseTimeout is phase execution timeout
	PhaseTimeout *time.Duration
	// Force forces phase execution
	Force *bool
	// Resume resumes failed install operation
	Resume *bool
	// Manual puts install operation in manual mode
	Manual *bool
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
	// DockerDevice is device to use for Docker data
	DockerDevice *string
	// SystemDevice is device to use for system data
	SystemDevice *string
	// ServerAddr is RPC server address
	ServerAddr *string
	// Mounts is additional app mounts
	Mounts *configure.KeyVal
	// CloudProvider turns on cloud provider integration
	CloudProvider *string
	// Manual turns on manual phases execution mode
	Manual *bool
	// Phase specifies the operation phase to execute
	Phase *string
	// PhaseTimeout is phase execution timeout
	PhaseTimeout *time.Duration
	// Resume resumes failed join operation
	Resume *bool
	// Force forces phase execution
	Force *bool
	// Complete marks join operation complete
	Complete *bool
	// OperationID is the ID of the operation created via UI
	OperationID *string
}

// AutoJoinCmd uses cloud provider info to join existing cluster
type AutoJoinCmd struct {
	*kingpin.CmdClause
	// ClusterName is cluster name
	ClusterName *string
	// Role is new node profile
	Role *string
	// DockerDevice is device to use for Docker data
	DockerDevice *string
	// SystemDevice is device to use for system data
	SystemDevice *string
	// Mounts is additional app mounts
	Mounts *configure.KeyVal
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

// PlanCmd displays operation plan
type PlanCmd struct {
	*kingpin.CmdClause
	// Init initializes the plan
	Init *bool
	// Sync the operation plan from etcd to local
	Sync *bool
	// Execute executes the given phase
	Execute *bool
	// Rollback reverses the given phase
	Rollback *bool
	// Resume resumes a paused (aborted) operation
	Resume *bool
	// Phase is the phase to execute
	Phase *string
	// Force forces execution of the given phase
	Force *bool
	// Output is output format
	Output *constants.Format
	// OperationID is optional ID of operation to show the plan for
	OperationID *string
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

// RollbackCmd rolls back the specified operation plan phase
type RollbackCmd struct {
	*kingpin.CmdClause
	// Phase is the phase to rollback
	Phase *string
	// PhaseTimeout is the rollback timeout
	PhaseTimeout *time.Duration
	// Force forces rollback
	Force *bool
	// SkipVersionCheck suppresses version mismatch errors
	SkipVersionCheck *bool
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
	// Complete marks upgrade as complete
	Complete *bool
	// Resume resumes failed upgrade
	Resume *bool
	// SkipVersionCheck suppresses version mismatch errors
	SkipVersionCheck *bool
}

// StatusCmd displays cluster status
type StatusCmd struct {
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

// StatusResetCmd resets cluster to active state
type StatusResetCmd struct {
	*kingpin.CmdClause
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
}

// AppCmd combines subcommands for app service
type AppCmd struct {
	*kingpin.CmdClause
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

// AppListCmd lists all apps
type AppListCmd struct {
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

// AppUninstallCmd launches application uninstall hook
type AppUninstallCmd struct {
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
	// ServiceUID is system user ID
	ServiceUID *string
	// ServiceGID is system user group ID
	ServiceGID *string
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
	// Args is additional arguments to the agent
	Args *[]string
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

// SystemCmd combines system subcommands
type SystemCmd struct {
	*kingpin.CmdClause
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
}

// SystemHistoryCmd displays system update history
type SystemHistoryCmd struct {
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

// SystemServiceStatusCmd displays status of systemd service
type SystemServiceStatusCmd struct {
	*kingpin.CmdClause
	// Package is system service package locator
	Package *loc.Locator
	// Name is service name
	Name *string
}

// SystemServiceListCmd lists systemd services
type SystemServiceListCmd struct {
	*kingpin.CmdClause
}

// SystemReportCmd generates tarball with system diagnostics information
type SystemReportCmd struct {
	*kingpin.CmdClause
	// Filter allows to collect only specific diagnostics
	Filter *[]string
	// Compressed allows to gzip the tarball
	Compressed *bool
}

// SystemStateDirCmd shows local state directory
type SystemStateDirCmd struct {
	*kingpin.CmdClause
}

// SystemDevicemapperCmd combines devicemapper related subcommands
type SystemDevicemapperCmd struct {
	*kingpin.CmdClause
}

// SystemDevicemapperMountCmd configures devicemapper environment
type SystemDevicemapperMountCmd struct {
	*kingpin.CmdClause
	// Disk is devicemapper device
	Disk *string
}

// SystemDevicemapperUnmountCmd removes devicemapper environment
type SystemDevicemapperUnmountCmd struct {
	*kingpin.CmdClause
}

// SystemDevicemapperSystemDirCmd show LVM system directory
type SystemDevicemapperSystemDirCmd struct {
	*kingpin.CmdClause
}

// SystemEnablePromiscModeCmd puts network interface into promiscuous mode
type SystemEnablePromiscModeCmd struct {
	*kingpin.CmdClause
	// Iface is interface to turn promiscuous mode on for
	Iface *string
}

// SystemDisablePromiscModeCmd removes promiscuous mode from interface
type SystemDisablePromiscModeCmd struct {
	*kingpin.CmdClause
	// Iface is interface to turn promiscuous mode off for
	Iface *string
}

// SystemExportRuntimeJournalCmd exports runtime journal to a file
type SystemExportRuntimeJournalCmd struct {
	*kingpin.CmdClause
	// OutputFile specifies the path of the resulting tarball
	OutputFile *string
}

// SystemStreamRuntimeJournalCmd streams contents of the runtime journal
type SystemStreamRuntimeJournalCmd struct {
	*kingpin.CmdClause
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

// GarbageCollectCmd prunes unused cluster resources
type GarbageCollectCmd struct {
	*kingpin.CmdClause
	// Phase is the specific phase to run
	Phase *string
	// PhaseTimeout is the phase execution timeout
	PhaseTimeout *time.Duration
	// Resume is whether to resume a failed garbage collection
	Resume *bool
	// Manual is whether the operation is not executed automatically
	Manual *bool
	// Confirmed is whether the user has confirmed the removal of custom docker
	// images
	Confirmed *bool
	// Force forces phase execution
	Force *bool
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
