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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/gravitational/configure"
	"github.com/gravitational/trace"
	"gopkg.in/alecthomas/kingpin.v2"
)

// RegisterCommands registers all gravity tool flags, arguments and subcommands
func RegisterCommands(app *kingpin.Application) *Application {
	g := &Application{Application: app}

	g.Debug = g.Flag("debug", "Enable debug mode").Bool()
	g.Silent = g.Flag("quiet", "Suppress any extra output to stdout").Short('q').Bool()
	g.Insecure = g.Flag("insecure", "Skip TLS verification").Default("false").Bool()
	g.StateDir = g.Flag("state-dir", "Directory for local state").String()
	g.EtcdRetryTimeout = g.Flag("etcd-retry-timeout", "Retry timeout for etcd transient errors").Hidden().Duration()
	g.UID = app.Flag("uid", "effective user ID for this operation. Must be >= 0").Default(strconv.Itoa(defaults.PlaceholderUserID)).Hidden().Int()
	g.GID = g.Flag("gid", "effective group ID for this operation. Must be >= 0").Default(strconv.Itoa(defaults.PlaceholderGroupID)).Hidden().Int()
	g.ProfileEndpoint = g.Flag("httpprofile", "enable profiling endpoint on specified host/port i.e. localhost:6060").Default("").Hidden().String()
	g.ProfileTo = g.Flag("profile-dir", "store periodic state snapshots in the specified directory").Default("").Hidden().String()
	g.UserLogFile = g.Flag("log-file", "log file with diagnostic information").Default(defaults.TelekubeUserLog).String()
	g.SystemLogFile = g.Flag("system-log-file", "log file with system level logs").Default(defaults.TelekubeSystemLog).Hidden().String()

	g.VersionCmd.CmdClause = g.Command("version", "Print gravity version")
	g.VersionCmd.Output = common.Format(g.VersionCmd.Flag("output", "Output format, text or json").Short('o').Default(string(constants.EncodingText)))

	g.InstallCmd.CmdClause = g.Command("install", "Install cluster on this node")
	g.InstallCmd.Path = g.InstallCmd.Arg("appdir", "Path to directory with application package. Uses current directory by default").String()
	g.InstallCmd.AdvertiseAddr = g.InstallCmd.Flag("advertise-addr", "The IP address to advertise").String()
	g.InstallCmd.Token = g.InstallCmd.Flag("token", "Unique install token to authorize other nodes to join the cluster").String()
	g.InstallCmd.CloudProvider = g.InstallCmd.Flag("cloud-provider", fmt.Sprintf("Cloud provider integration: %v. If not set, autodetect environment", schema.SupportedProviders)).String()
	g.InstallCmd.Cluster = g.InstallCmd.Flag("cluster", "Cluster name, optional").String()
	g.InstallCmd.App = g.InstallCmd.Flag("app", "Application to install, optional").Hidden().String()
	g.InstallCmd.Flavor = g.InstallCmd.Flag("flavor", "Application flavor, optional").String()
	g.InstallCmd.Role = g.InstallCmd.Flag("role", "Role of this node, optional").String()
	g.InstallCmd.ResourcesPath = g.InstallCmd.Flag("config", "Kubernetes configuration resources, will be injected at cluster creation time").String()
	g.InstallCmd.Wizard = g.InstallCmd.Flag("wizard", "(Obsolete, superseded by 'mode') Start installer with web wizard interface").Bool()
	g.InstallCmd.Mode = g.InstallCmd.Flag("mode", fmt.Sprintf("Install mode, one of %v", modules.Get().InstallModes())).Default(constants.InstallModeCLI).Hidden().String()
	g.InstallCmd.DockerDevice = g.InstallCmd.Flag("docker-device", "Device to use for docker storage").Hidden().String()
	g.InstallCmd.SystemDevice = g.InstallCmd.Flag("system-device", "Device to use for system data directory").Hidden().String()
	g.InstallCmd.Mounts = configure.KeyValParam(g.InstallCmd.Flag("mount", "One or several mounts in form <mount-name>:<path>, e.g. data:/var/lib/data"))
	g.InstallCmd.PodCIDR = g.InstallCmd.Flag("pod-network-cidr", "Subnet range for pods. Must be a minimum of /16").Default(defaults.PodSubnet).String()
	g.InstallCmd.ServiceCIDR = g.InstallCmd.Flag("service-cidr", "Subnet range for services").Default(defaults.ServiceSubnet).String()
	g.InstallCmd.VxlanPort = g.InstallCmd.Flag("vxlan-port", "Custom overlay network port").Default(strconv.Itoa(defaults.VxlanPort)).Int()
	g.InstallCmd.DNSListenAddrs = g.InstallCmd.Flag("dns-listen-addr", "Custom listen address for in-cluster DNS").
		Default(defaults.DNSListenAddr).IPList()
	g.InstallCmd.DNSPort = g.InstallCmd.Flag("dns-port", "Custom DNS port for in-cluster DNS").
		Default(strconv.Itoa(defaults.DNSPort)).Int()
	g.InstallCmd.DockerStorageDriver = DockerStorageDriver(g.InstallCmd.Flag("storage-driver",
		fmt.Sprintf("Docker storage driver, overrides the one from app manifest. Recognized are: %v", strings.Join(constants.DockerSupportedDrivers, ", "))), constants.DockerSupportedDrivers)
	g.InstallCmd.DockerArgs = g.InstallCmd.Flag("docker-opt", "Additional arguments to docker. Can be specified multiple times").Strings()
	g.InstallCmd.Phase = g.InstallCmd.Flag("phase", "Execute an install plan phase").String()
	g.InstallCmd.PhaseTimeout = g.InstallCmd.Flag("timeout", "Phase execution timeout").Default(defaults.PhaseTimeout).Hidden().Duration()
	g.InstallCmd.Force = g.InstallCmd.Flag("force", "Force phase execution").Bool()
	g.InstallCmd.Resume = g.InstallCmd.Flag("resume", "Resume installation from last failed step").Bool()
	g.InstallCmd.Manual = g.InstallCmd.Flag("manual", "Manually execute install operation phases").Bool()
	g.InstallCmd.ServiceUID = g.InstallCmd.Flag("service-uid",
		fmt.Sprintf("Service user ID for planet. %q user will created and used if none specified", defaults.ServiceUser)).
		Default(defaults.ServiceUserID).
		OverrideDefaultFromEnvar(constants.ServiceUserEnvVar).
		String()
	g.InstallCmd.ServiceGID = g.InstallCmd.Flag("service-gid",
		fmt.Sprintf("Service group ID for planet. %q group will created and used if none specified", defaults.ServiceUserGroup)).
		Default(defaults.ServiceGroupID).
		OverrideDefaultFromEnvar(constants.ServiceGroupEnvVar).
		String()
	g.InstallCmd.GCENodeTags = g.InstallCmd.Flag("gce-node-tag", "Override node tag on the instance in GCE required for load balanacing. Defaults to cluster name.").Strings()
	g.InstallCmd.DNSHosts = g.InstallCmd.Flag("dns-host", "Specify an IP address that will be returned for the given domain within the cluster. Accepts <domain>/<ip> format. Can be specified multiple times.").Hidden().Strings()
	g.InstallCmd.DNSZones = g.InstallCmd.Flag("dns-zone", "Specify an upstream server for the given zone within the cluster. Accepts <zone>/<nameserver> format where <nameserver> can be either <ip> or <ip>:<port>. Can be specified multiple times.").Strings()

	g.JoinCmd.CmdClause = g.Command("join", "Join existing cluster or on-going install operation")
	g.JoinCmd.PeerAddr = g.JoinCmd.Arg("peer-addrs", "One or several IP addresses of cluster node to join, as comma-separated values").String()
	g.JoinCmd.AdvertiseAddr = g.JoinCmd.Flag("advertise-addr", "IP address to advertise").String()
	g.JoinCmd.Token = g.JoinCmd.Flag("token", "Unique install token to authorize this node to join the cluster").String()
	g.JoinCmd.Role = g.JoinCmd.Flag("role", "Role of this node, optional").String()
	g.JoinCmd.DockerDevice = g.JoinCmd.Flag("docker-device", "Docker device to use").Hidden().String()
	g.JoinCmd.SystemDevice = g.JoinCmd.Flag("system-device", "Device to use for system data directory").Hidden().String()
	g.JoinCmd.ServerAddr = g.JoinCmd.Flag("server-addr", "Address of the agent server").Hidden().String()
	g.JoinCmd.Mounts = configure.KeyValParam(g.JoinCmd.Flag("mount", "One or several mounts in form <mount-name>:<path>, e.g. data:/var/lib/data"))
	g.JoinCmd.CloudProvider = g.JoinCmd.Flag("cloud-provider", "Cloud provider integration e.g. 'generic', 'aws'. If not set, autodetect environment").String()
	g.JoinCmd.Manual = g.JoinCmd.Flag("manual", "Manually execute join operation phases").Bool()
	g.JoinCmd.Phase = g.JoinCmd.Flag("phase", "Execute specific operation phase").String()
	g.JoinCmd.PhaseTimeout = g.JoinCmd.Flag("timeout", "Phase execution timeout").Default(defaults.PhaseTimeout).Hidden().Duration()
	g.JoinCmd.Resume = g.JoinCmd.Flag("resume", "Resume joining from last failed step").Bool()
	g.JoinCmd.Force = g.JoinCmd.Flag("force", "Force phase execution").Bool()
	g.JoinCmd.Complete = g.JoinCmd.Flag("complete", "Complete join operation").Bool()
	g.JoinCmd.OperationID = g.JoinCmd.Flag("operation-id", "ID of the operation that was created via UI").Hidden().String()

	g.AutoJoinCmd.CmdClause = g.Command("autojoin", "Use cloud provider data to join a node to existing cluster")
	g.AutoJoinCmd.ClusterName = g.AutoJoinCmd.Arg("cluster-name", "Cluster name used for discovery").Required().String()
	g.AutoJoinCmd.Role = g.AutoJoinCmd.Flag("role", "Role of this node, optional").String()
	g.AutoJoinCmd.DockerDevice = g.AutoJoinCmd.Flag("docker-device", "Docker device to use").Hidden().String()
	g.AutoJoinCmd.SystemDevice = g.AutoJoinCmd.Flag("system-device", "Device to use for system data directory").Hidden().String()
	g.AutoJoinCmd.Mounts = configure.KeyValParam(g.AutoJoinCmd.Flag("mount", "One or several mounts in form <mount-name>:<path>, e.g. data:/var/lib/data"))

	g.LeaveCmd.CmdClause = g.Command("leave", "Decommission this node from the cluster")
	g.LeaveCmd.Force = g.LeaveCmd.Flag("force", "Force local state cleanup").Bool()
	g.LeaveCmd.Confirm = g.LeaveCmd.Flag("confirm", "Do not ask for confirmation").Bool()

	g.RemoveCmd.CmdClause = g.Command("remove", "Remove a node from the cluster")
	g.RemoveCmd.Node = g.RemoveCmd.Arg("node", "Node to remove: can be IP address, hostname or name from `kubectl get nodes` output)").
		Required().String()
	g.RemoveCmd.Force = g.RemoveCmd.Flag("force", "Force removal of offline node").Bool()
	g.RemoveCmd.Confirm = g.RemoveCmd.Flag("confirm", "Do not ask for confirmation").Bool()

	g.PlanCmd.CmdClause = g.Command("plan", "Display a plan for an ongoing operation")
	g.PlanCmd.Init = g.PlanCmd.Flag("init", "Initialize operation plan").Bool()
	g.PlanCmd.Sync = g.PlanCmd.Flag("sync", "Sync the operation plan from etcd to local store").Bool()
	g.PlanCmd.Execute = g.PlanCmd.Flag("execute", "Execute specified operation phase").Bool()
	g.PlanCmd.Rollback = g.PlanCmd.Flag("rollback", "Rollback specified operation phase").Bool()
	g.PlanCmd.Resume = g.PlanCmd.Flag("resume", "Resume last aborted operation").Bool()
	g.PlanCmd.Force = g.PlanCmd.Flag("force", "Force execution of specified phase").Bool()
	g.PlanCmd.Phase = g.PlanCmd.Flag("phase", "Phase ID to execute").String()
	g.PlanCmd.Output = common.Format(g.PlanCmd.Flag("output", "Output format for the plan, text, json or yaml").Short('o').Default(string(constants.EncodingText)))
	g.PlanCmd.OperationID = g.PlanCmd.Flag("operation-id", "ID of the operation to display the plan for. It not specified, the last operation plan will be displayed").String()
	g.PlanCmd.SkipVersionCheck = g.PlanCmd.Flag("skip-version-check", "Bypass version compatibility check").Hidden().Bool()
	g.PlanCmd.PhaseTimeout = g.PlanCmd.Flag("timeout", "Phase rollback timeout").Default(defaults.PhaseTimeout).Hidden().Duration()

	g.UpdateCmd.CmdClause = g.Command("update", "Update actions on cluster")

	g.UpdateCheckCmd.CmdClause = g.UpdateCmd.Command("check", "Check if an update is available for the specified application").Hidden()
	g.UpdateCheckCmd.App = g.UpdateCheckCmd.Arg("app", "Application version to update to, in the 'name:version' or 'name' (for latest version) format. If unspecified, currently installed application is updated").String()

	g.UpdateTriggerCmd.CmdClause = g.UpdateCmd.Command("trigger", "Trigger an update operation for given application").Hidden()
	g.UpdateTriggerCmd.App = g.UpdateTriggerCmd.Arg("app", "Application version to update to, in the 'name:version' or 'name' (for latest version) format. If unspecified, currently installed application is updated").String()
	g.UpdateTriggerCmd.Manual = g.UpdateTriggerCmd.Flag("manual", "Manual operation. Do not trigger automatic update").Short('m').Bool()

	// upgrade is aliased to "update trigger"
	g.UpgradeCmd.CmdClause = g.Command("upgrade", "Trigger an update operation for given application").Hidden()
	g.UpgradeCmd.App = g.UpgradeCmd.Arg("app", "Application version to update to, in the 'name:version' or 'name' (for latest version) format. If unspecified, currently installed application is updated").String()
	g.UpgradeCmd.Manual = g.UpgradeCmd.Flag("manual", "Manual upgrade mode").Short('m').Bool()
	g.UpgradeCmd.Phase = g.UpgradeCmd.Flag("phase", "Operation phase to execute").String()
	g.UpgradeCmd.Timeout = g.UpgradeCmd.Flag("timeout", "Phase execution timeout").Default(defaults.PhaseTimeout).Hidden().Duration()
	g.UpgradeCmd.Force = g.UpgradeCmd.Flag("force", "Force phase execution even if pre-conditions are not satisfied").Bool()
	g.UpgradeCmd.Complete = g.UpgradeCmd.Flag("complete", "Complete update operation").Bool()
	g.UpgradeCmd.Resume = g.UpgradeCmd.Flag("resume", "Resume upgrade from the last failed step").Bool()
	g.UpgradeCmd.SkipVersionCheck = g.UpgradeCmd.Flag("skip-version-check", "Bypass version compatibility check").Hidden().Bool()

	g.UpdateUploadCmd.CmdClause = g.UpdateCmd.Command("upload", "Upload update package to locally running site").Hidden()
	g.UpdateUploadCmd.OpsCenterURL = g.UpdateUploadCmd.Flag("ops-url", "Optional OpsCenter URL to upload new packages to (defaults to local gravity site)").Default(defaults.GravityServiceURL).String()

	// manual update flow commands
	g.UpdateCompleteCmd.CmdClause = g.UpdateCmd.Command("complete", "Mark update operation as completed").Hidden()
	g.UpdateCompleteCmd.Failed = g.UpdateCompleteCmd.Flag("failed", "Mark update operation as failure").Short('f').Bool()

	// Alias for `gravity system update`
	g.UpdateSystemCmd.CmdClause = g.UpdateCmd.Command("system", "Update this system by installing newer versions of system packages").Hidden()
	g.UpdateSystemCmd.ChangesetID = g.UpdateSystemCmd.Flag("changeset-id", "Assign ID to this update operation (will be autogenerated if missing)").Hidden().String()
	g.UpdateSystemCmd.ServiceName = g.UpdateSystemCmd.Flag("service-name", "The name of the service to run update as a systemd unit").Hidden().String()
	g.UpdateSystemCmd.WithStatus = g.UpdateSystemCmd.Flag("with-status", "Verify the system status at the end of the operation").Bool()
	g.UpdateSystemCmd.RuntimePackage = Locator(g.UpdateSystemCmd.Flag("runtime-package", "The name of the runtime package to update to").Required())

	g.StatusCmd.CmdClause = g.Command("status", "Show the status of the cluster and the application running in it")
	g.StatusCmd.Token = g.StatusCmd.Flag("token", "Show only the cluster token").Bool()
	g.StatusCmd.Tail = g.StatusCmd.Flag("tail", "Tail the logs of the currently running operation until it completes").Bool()
	g.StatusCmd.OperationID = g.StatusCmd.Flag("operation-id", "Check status of operation with given ID").Short('o').String()
	g.StatusCmd.Seconds = g.StatusCmd.Flag("seconds", "Continuously display status every N seconds").Short('s').Int()
	g.StatusCmd.Output = common.Format(g.StatusCmd.Flag("output", "output format: json or text").Default(string(constants.EncodingText)))

	// reset cluster state, for debugging/emergencies
	g.StatusResetCmd.CmdClause = g.Command("status-reset", "Reset the cluster state to 'active'").Hidden()

	// backup
	g.BackupCmd.CmdClause = g.Command("backup", "Backup the local application state")
	g.BackupCmd.Tarball = g.BackupCmd.Arg("to", "Tarball to create with results of the backup hook").Required().String()
	g.BackupCmd.Timeout = g.BackupCmd.Flag("timeout", "Active deadline for the backup job, in Go duration format (e.g. 30s, 5m, etc.). If not specified, the value from manifest is used. If that is not specified as well, the default value of 20 minutes is used").Duration()
	g.BackupCmd.Follow = g.BackupCmd.Flag("follow", "Output backup job logs to the stdout").Bool()

	g.CheckCmd.CmdClause = g.Command("check", "check host environment to match manifest")
	g.CheckCmd.ManifestFile = g.CheckCmd.Arg("manifest", "application manifest in YAML format").Default(defaults.ManifestFileName).String()
	g.CheckCmd.Profile = g.CheckCmd.Flag("profile", "profile to check").Short('p').Required().String()
	g.CheckCmd.AutoFix = g.CheckCmd.Flag("autofix", "attempt to fix some of the problems").Bool()

	// restore
	g.RestoreCmd.CmdClause = g.Command("restore", "Restore state of the local application from a previously taken backup")
	g.RestoreCmd.Tarball = g.RestoreCmd.Arg("from", "Tarball with backup data to restore from").Required().String()
	g.RestoreCmd.Follow = g.RestoreCmd.Flag("follow", "Output restore job logs to the stdout").Bool()
	g.RestoreCmd.Timeout = g.RestoreCmd.Flag("timeout", fmt.Sprintf("Maximum time a restore job is active. Defaults to the value from the manifest or %v if unspecified", defaults.HookJobDeadline)).Duration()

	// operations on gravity applications
	g.AppCmd.CmdClause = g.Command("app", "operations on gravity applications")

	// import gravity application
	g.AppImportCmd.CmdClause = g.AppCmd.Command("import", "Import application into gravity").Hidden()
	g.AppImportCmd.Source = g.AppImportCmd.Arg("src", "path to application resources (directory / file)").Required().String()
	g.AppImportCmd.Repository = g.AppImportCmd.Flag("repository", "optional repository name, overrides the one specified in the app manifest").String()
	g.AppImportCmd.Name = g.AppImportCmd.Flag("name", "optional app name, overrides the one specified in the app manifest").String()
	g.AppImportCmd.Version = g.AppImportCmd.Flag("version", "optional app version, overrides the one specified in the app manifest").String()
	g.AppImportCmd.RegistryURL = g.AppImportCmd.Flag("registry-url", "optional remote docker registry URL").Default(constants.DockerRegistry).String()
	g.AppImportCmd.DockerURL = g.AppImportCmd.Flag("docker-url", "optional docker URL").Default(constants.DockerEngineURL).String()
	g.AppImportCmd.OpsCenterURL = g.AppImportCmd.Flag("ops-url", "optional OpsCenter URL").String()
	g.AppImportCmd.Vendor = g.AppImportCmd.Flag("vendor", "rewrite all container images to use private docker registry (requires --registry-url)").Bool()
	g.AppImportCmd.Force = g.AppImportCmd.Flag("force", "overwrite existing application").Bool()
	g.AppImportCmd.Excludes = g.AppImportCmd.Flag("exclude", "exclusion patterns for resulting tarball").Strings()
	g.AppImportCmd.IncludePaths = g.AppImportCmd.Flag("include", "include paths for resulting tarball").Strings()
	g.AppImportCmd.VendorPatterns = g.AppImportCmd.Flag("glob", "file pattern to search for container image references").Default(defaults.VendorPattern).Strings()
	g.AppImportCmd.VendorIgnorePatterns = g.AppImportCmd.Flag("ignore", "ignore files matching this regular expression when searching for container references").Strings()
	g.AppImportCmd.SetImages = loc.ImagesSlice(g.AppImportCmd.Flag("set-image", "rewrite docker image versions in the app's resource files during vendoring, e.g. 'postgres:9.3.4' will rewrite all images with name 'postgres' to 'postgres:9.3.4'"))
	g.AppImportCmd.SetDeps = loc.LocatorSlice(g.AppImportCmd.Flag("set-dep", "rewrite dependencies section in app's manifest file during vendoring, e.g. 'gravitational.io/site-app:0.0.39' will overwrite dependency to 'gravitational.io/site-app:0.0.39'"))
	g.AppImportCmd.Parallel = g.AppImportCmd.Flag("parallel", "specifies number of concurrent tasks. If < 0, the number of tasks is not restricted, if unspecified, then tasks are capped at the number of logical CPU cores.").Hidden().Int()

	// export gravity application
	g.AppExportCmd.CmdClause = g.AppCmd.Command("export", "export gravity application").Hidden()
	g.AppExportCmd.Locator = g.AppExportCmd.Arg("pkg", "package name with application to export").Required().String()
	g.AppExportCmd.RegistryURL = g.AppExportCmd.Flag("registry-url", "docker registry URL to use for export").Default(constants.DockerRegistry).String()
	g.AppExportCmd.OpsCenterURL = g.AppExportCmd.Flag("ops-url", "optional remote opscenter URL").String()

	// delete gravity application
	g.AppDeleteCmd.CmdClause = g.AppCmd.Command("delete", "delete gravity application").Hidden()
	g.AppDeleteCmd.Locator = g.AppDeleteCmd.Arg("pkg", "application package name to delete").Required().String()
	g.AppDeleteCmd.OpsCenterURL = g.AppDeleteCmd.Flag("ops-url", "optional remote OpsCenter URL").String()
	g.AppDeleteCmd.Force = g.AppDeleteCmd.Flag("force", "do not produce error if app does not exist").Bool()

	// list installed apps
	g.AppListCmd.CmdClause = g.AppCmd.Command("list", "list installed applications").Hidden()
	g.AppListCmd.Repository = g.AppListCmd.Arg("repo", "list applications in the specified repository").String()
	g.AppListCmd.Type = g.AppListCmd.Flag("type", "restrict applications to the specified type").String()
	g.AppListCmd.ShowHidden = g.AppListCmd.Flag("hidden", "show hidden apps too").Hidden().Bool()
	g.AppListCmd.OpsCenterURL = g.AppListCmd.Flag("ops-url", "optional remote Ops Center URL").String()

	// uninstall app
	g.AppUninstallCmd.CmdClause = g.AppCmd.Command("uninstall", "uninstall application").Hidden()
	g.AppUninstallCmd.Locator = Locator(g.AppUninstallCmd.Arg("pkg", "package name with application").Required())

	// get status of an application
	g.AppStatusCmd.CmdClause = g.AppCmd.Command("status", "get app status").Hidden()
	g.AppStatusCmd.Locator = Locator(g.AppStatusCmd.Arg("pkg", "application package").Required())
	g.AppStatusCmd.OpsCenterURL = g.AppStatusCmd.Flag("ops-url", "optional remote OpsCenter").String()

	// pull an application from a remote OpsCenter
	g.AppPullCmd.CmdClause = g.AppCmd.Command("pull", "pull an application package from remote OpsCenter").Hidden()
	g.AppPullCmd.Package = Locator(g.AppPullCmd.Arg("pkg", "application package").Required())
	g.AppPullCmd.OpsCenterURL = g.AppPullCmd.Flag("ops-url", "remote OpsCenter URL").Required().String()
	g.AppPullCmd.Labels = configure.KeyValParam(g.AppPullCmd.Flag("labels", "labels to add to the package"))
	g.AppPullCmd.Force = g.AppPullCmd.Flag("force", "overwrite destination app if it already exists").Bool()

	// push an application to a remote OpsCenter
	g.AppPushCmd.CmdClause = g.AppCmd.Command("push", "push an application package to remote OpsCenter").Hidden()
	g.AppPushCmd.Package = Locator(g.AppPushCmd.Arg("pkg", "application package").Required())
	g.AppPushCmd.OpsCenterURL = g.AppPushCmd.Flag("ops-url", "remote ops center url").Required().String()

	// run an application hook
	g.AppHookCmd.CmdClause = g.AppCmd.Command("hook", "run the specified application hook").Hidden()
	g.AppHookCmd.Package = Locator(g.AppHookCmd.Arg("pkg", "application package").Required())
	g.AppHookCmd.HookName = g.AppHookCmd.Arg("hook-name", fmt.Sprintf("name of the hook (one of %v)", schema.AllHooks())).Required().String()
	g.AppHookCmd.Env = g.AppHookCmd.Flag("env", "additional environment variables to provide to hook job as key=value pairs. Can be specified multiple times").StringMap()

	// unpack application resources
	g.AppUnpackCmd.CmdClause = g.AppCmd.Command("unpack", "unpack application resources").Hidden()
	g.AppUnpackCmd.Package = Locator(g.AppUnpackCmd.Arg("pkg", "application package").Required())
	g.AppUnpackCmd.Dir = g.AppUnpackCmd.Arg("dir", "output directory").Required().String()
	g.AppUnpackCmd.OpsCenterURL = g.AppUnpackCmd.Flag("ops-url", "optional remote OpsCenter URL").String()
	g.AppUnpackCmd.ServiceUID = g.AppUnpackCmd.Flag("service-uid", "optional service user ID").String()

	g.WizardCmd.CmdClause = g.Command("wizard", "start wizard that will guide you through install process").Hidden()
	g.WizardCmd.ServiceUID = g.WizardCmd.Flag("service-uid", fmt.Sprintf("Service user ID for planet. %q user will created and used if none specified", defaults.ServiceUser)).Default(defaults.ServiceUserID).OverrideDefaultFromEnvar(constants.ServiceUserEnvVar).String()
	g.WizardCmd.ServiceGID = g.WizardCmd.Flag("service-gid", fmt.Sprintf("Service group ID for planet. %q group will created and used if none specified", defaults.ServiceUserGroup)).Default(defaults.ServiceGroupID).OverrideDefaultFromEnvar(constants.ServiceGroupEnvVar).String()

	g.AppPackageCmd.CmdClause = g.Command("app-package", "Display the name of application package from installer tarball").Hidden()

	// install and access ops commands
	g.OpsCmd.CmdClause = g.Command("ops", "access OpsCenter related commands")

	g.OpsConnectCmd.CmdClause = g.OpsCmd.Command("connect", "save credentials for remote OpsCenter on local disk").Hidden()
	g.OpsConnectCmd.OpsCenterURL = g.OpsConnectCmd.Arg("ops-url", "remote OpsCenter URL").Default(defaults.GravityServiceURL).String()
	g.OpsConnectCmd.Username = g.OpsConnectCmd.Arg("username", "remote OpsCenter username").String()
	g.OpsConnectCmd.Password = g.OpsConnectCmd.Arg("password", "remote OpsCenter password").String()

	g.OpsDisconnectCmd.CmdClause = g.OpsCmd.Command("disconnect", "disconnect and log out from OpsCenter").Hidden()
	g.OpsDisconnectCmd.OpsCenterURL = g.OpsDisconnectCmd.Arg("ops-url", "remote OpsCenter URL").Required().String()

	g.OpsListCmd.CmdClause = g.OpsCmd.Command("ls", "list connected OpsCenters").Hidden()

	// TODO: move this functionality to crpcAgent
	g.OpsAgentCmd.CmdClause = g.OpsCmd.Command("agent", "Start an agent to perform a set of tasks").Hidden()
	g.OpsAgentCmd.PackageAddr = g.OpsAgentCmd.Arg("package-addr", "Address of the package service").Required().String()
	g.OpsAgentCmd.AdvertiseAddr = g.OpsAgentCmd.Flag("advertise-addr", "IP address to advertise").Required().IP()
	g.OpsAgentCmd.ServerAddr = g.OpsAgentCmd.Flag("server-addr", "Address of the agent server").Required().String()
	g.OpsAgentCmd.Token = g.OpsAgentCmd.Flag("token", "Unique token to authorize the agent to the server").Required().String()
	g.OpsAgentCmd.ServiceName = g.OpsAgentCmd.Flag("service-name", "Start agent in a systemd service with this name").String()
	g.OpsAgentCmd.Vars = configure.KeyValParam(g.OpsAgentCmd.Flag("vars", "Additional attributes as key=value pairs"))
	g.OpsAgentCmd.ServiceUID = g.OpsAgentCmd.Flag("service-uid", fmt.Sprintf("Service user ID for planet. %q user will created and used if none specified", defaults.ServiceUser)).Default(defaults.ServiceUserID).OverrideDefaultFromEnvar(constants.ServiceUserEnvVar).String()
	g.OpsAgentCmd.ServiceGID = g.OpsAgentCmd.Flag("service-gid", fmt.Sprintf("Service group ID for planet. %q group will created and used if none specified", defaults.ServiceUserGroup)).Default(defaults.ServiceGroupID).OverrideDefaultFromEnvar(constants.ServiceGroupEnvVar).String()
	g.OpsAgentCmd.CloudProvider = g.OpsAgentCmd.Flag("cloud-provider", "Cloud provider integration e.g. 'generic', 'aws'. If not set, autodetect environment").String()

	// operations on packages
	g.PackCmd.CmdClause = g.Command("package", "operations on gravity system packages")

	// import package
	g.PackImportCmd.CmdClause = g.PackCmd.Command("import", "import file or directory into package").Hidden()
	g.PackImportCmd.CheckManifest = g.PackImportCmd.Flag("check-manifest", "check manifest in the package").Bool()
	g.PackImportCmd.OpsCenterURL = g.PackImportCmd.Flag("ops-url", "remote OpsCenter URL").String()
	g.PackImportCmd.Path = g.PackImportCmd.Arg("path", "file or directory to import as a package").Required().ExistingFileOrDir()
	g.PackImportCmd.Locator = Locator(g.PackImportCmd.Arg("pkg", "package name").Required())
	g.PackImportCmd.Labels = configure.KeyValParam(g.PackImportCmd.Flag("labels", "labels to add to the package"))

	// unpack package
	g.PackUnpackCmd.CmdClause = g.PackCmd.Command("unpack", "unpack package into internal 'unpacked' directory").Hidden()
	g.PackUnpackCmd.Locator = Locator(g.PackUnpackCmd.Arg("pkg", "package name").Required())
	g.PackUnpackCmd.Dir = g.PackUnpackCmd.Arg("dir", "output unpack directory").String()
	g.PackUnpackCmd.OpsCenterURL = g.PackUnpackCmd.Flag("ops-url", "optional remote OpsCenter URL").String()

	// export package
	g.PackExportCmd.CmdClause = g.PackCmd.Command("export", "export package to specified file").Hidden()
	g.PackExportCmd.Locator = Locator(g.PackExportCmd.Arg("pkg", "package name").Required())
	g.PackExportCmd.File = g.PackExportCmd.Arg("file", "output file with a package").Required().String()
	g.PackExportCmd.OpsCenterURL = g.PackExportCmd.Flag("ops-url", "optional remote OpsCenter URL").String()
	g.PackExportCmd.FileMask = g.PackExportCmd.Flag("file-mask", "optional output file access mode (octal, as specified with chmod)").Default(strconv.FormatUint(defaults.SharedReadWriteMask, 8)).String()

	// list packages
	g.PackListCmd.CmdClause = g.PackCmd.Command("list", "list local packages").Hidden()
	g.PackListCmd.Repository = g.PackListCmd.Arg("repository", "repository name, if omitted will list all packages").String()
	g.PackListCmd.OpsCenterURL = g.PackListCmd.Flag("ops-url", "optional remote OpsCenter URL").String()

	// delete package
	g.PackDeleteCmd.CmdClause = g.PackCmd.Command("delete", "delete a package from repository").Hidden()
	g.PackDeleteCmd.Force = g.PackDeleteCmd.Flag("force", "force deletion (ignore errors if not exists)").Bool()
	g.PackDeleteCmd.Locator = Locator(g.PackDeleteCmd.Arg("pkg", "package name"))
	g.PackDeleteCmd.OpsCenterURL = g.PackDeleteCmd.Flag("ops-url", "optional remote OpsCenter URL").String()

	// configure package
	g.PackConfigureCmd.CmdClause = g.PackCmd.Command("configure", "configure a package").Interspersed(false).Hidden()
	g.PackConfigureCmd.Package = Locator(g.PackConfigureCmd.Arg("pkg", "package name to configure").Required())
	g.PackConfigureCmd.ConfPackage = Locator(g.PackConfigureCmd.Arg("conf-pkg", "package name that captures resulting configuration").Required())
	g.PackConfigureCmd.Args = g.PackConfigureCmd.Arg("arg", "additional arguments to command").Strings()

	// execute command provided by package
	g.PackCommandCmd.CmdClause = g.PackCmd.Command("command", "execute command provided by the package").Interspersed(false).Hidden()
	g.PackCommandCmd.Command = g.PackCommandCmd.Arg("cmd", "command to execute").Required().String()
	g.PackCommandCmd.Package = Locator(g.PackCommandCmd.Arg("pkg", "package name to execute").Required())
	g.PackCommandCmd.ConfPackage = Locator(g.PackCommandCmd.Arg("conf-pkg", "package with config"))
	g.PackCommandCmd.Args = g.PackCommandCmd.Arg("arg", "additional arguments to command").Strings()

	// push package to remote OpsCenter
	g.PackPushCmd.CmdClause = g.PackCmd.Command("push", "push package to remote OpsCenter").Hidden()
	g.PackPushCmd.Package = Locator(g.PackPushCmd.Arg("pkg", "package name to push").Required())
	g.PackPushCmd.OpsCenterURL = g.PackPushCmd.Flag("ops-url", "optional remote OpsCenter URL").String()

	// pull package from remote OpsCenter
	g.PackPullCmd.CmdClause = g.PackCmd.Command("pull", "pull package from remote OpsCenter").Hidden()
	g.PackPullCmd.Package = Locator(g.PackPullCmd.Arg("pkg", "package name to pull").Required())
	g.PackPullCmd.OpsCenterURL = g.PackPullCmd.Flag("ops-url", "remote OpsCenter URL").String()
	g.PackPullCmd.Labels = configure.KeyValParam(g.PackPullCmd.Flag("labels", "labels to add to the package"))
	g.PackPullCmd.Force = g.PackPullCmd.Flag("force", "overwrite destination package if it already exists").Bool()

	// labels changes package labels
	g.PackLabelsCmd.CmdClause = g.PackCmd.Command("labels", "change package labels").Hidden()
	g.PackLabelsCmd.Package = Locator(g.PackLabelsCmd.Arg("pkg", "package name to change").Required())
	g.PackLabelsCmd.OpsCenterURL = g.PackLabelsCmd.Flag("ops-url", "remote OpsCenter URL").String()
	g.PackLabelsCmd.Add = configure.KeyValParam(g.PackLabelsCmd.Flag("add", "labels to add to the package"))
	g.PackLabelsCmd.Remove = g.PackLabelsCmd.Flag("remove", "labels to remove from the package").Strings()

	// operations with users
	g.UserCmd.CmdClause = g.Command("user", "operations with gravity users, only agent users are supported")

	// create a new user
	g.UserCreateCmd.CmdClause = g.UserCmd.Command("create", "create a new user").Hidden()
	g.UserCreateCmd.Email = g.UserCreateCmd.Flag("email", "user email").Required().String()
	g.UserCreateCmd.Type = g.UserCreateCmd.Flag("type", "agent, remote_agent or admin").Default("agent").String()
	g.UserCreateCmd.Password = g.UserCreateCmd.Flag("password", "user password, mandatory for admin").String()
	g.UserCreateCmd.OpsCenterURL = g.UserCreateCmd.Flag("ops-url", "remote OpsCenter URL").Required().String()

	// delete a user
	g.UserDeleteCmd.CmdClause = g.UserCmd.Command("delete", "delete a user").Hidden()
	g.UserDeleteCmd.Email = g.UserDeleteCmd.Flag("email", "user email").Required().String()
	g.UserDeleteCmd.OpsCenterURL = g.UserDeleteCmd.Flag("ops-url", "remote OpsCenter URL").Required().String()

	g.UsersCmd.CmdClause = g.Command("users", "Manage user accounts")

	// create a user invite
	g.UsersInviteCmd.CmdClause = g.UsersCmd.Command("add", "Generate a user invitation token")
	g.UsersInviteCmd.Name = g.UsersInviteCmd.Arg("account", "User account name").Required().String()
	g.UsersInviteCmd.Roles = g.UsersInviteCmd.Flag("roles", "List of roles for the new user to assume").Required().Strings()
	g.UsersInviteCmd.TTL = g.UsersInviteCmd.Flag("ttl",
		fmt.Sprintf("Set expiration time for token, default is %v hour, maximum is %v hours",
			int(defaults.SignupTokenTTL/time.Hour),
			int(defaults.MaxSignupTokenTTL/time.Hour))).
		Default(fmt.Sprintf("%v", defaults.SignupTokenTTL)).Duration()

	// reset a user
	g.UsersResetCmd.CmdClause = g.UsersCmd.Command("reset", "Reset user password and generate a new token")
	g.UsersResetCmd.Name = g.UsersResetCmd.Arg("account", "User account name").Required().String()
	g.UsersResetCmd.TTL = g.UsersResetCmd.Flag("ttl",
		fmt.Sprintf("Set expiration time for token, default is %v hour, maximum is %v hours",
			int(defaults.UserResetTokenTTL/time.Hour),
			int(defaults.MaxUserResetTokenTTL/time.Hour))).
		Default(fmt.Sprintf("%v", defaults.UserResetTokenTTL)).Duration()

	// operations with api keys
	g.APIKeyCmd.CmdClause = g.Command("apikey", "operations with api keys")

	// create a new api key
	g.APIKeyCreateCmd.CmdClause = g.APIKeyCmd.Command("create", "create a new api key").Hidden()
	g.APIKeyCreateCmd.Email = g.APIKeyCreateCmd.Flag("email", "email of the agent user to create an api key for").Required().String()
	g.APIKeyCreateCmd.OpsCenterURL = g.APIKeyCreateCmd.Flag("ops-url", "remote OpsCenter URL").Required().String()

	// view api keys for a user
	g.APIKeyListCmd.CmdClause = g.APIKeyCmd.Command("list", "view user api keys").Hidden()
	g.APIKeyListCmd.Email = g.APIKeyListCmd.Flag("email", "email of the user to view api keys for").Required().String()
	g.APIKeyListCmd.OpsCenterURL = g.APIKeyListCmd.Flag("ops-url", "remote OpsCenter URL").Required().String()

	// delete an api key
	g.APIKeyDeleteCmd.CmdClause = g.APIKeyCmd.Command("delete", "delete an api key").Hidden()
	g.APIKeyDeleteCmd.Token = g.APIKeyDeleteCmd.Arg("token", "api key to delete").Required().String()
	g.APIKeyDeleteCmd.Email = g.APIKeyDeleteCmd.Arg("email", "email of the user").Required().String()
	g.APIKeyDeleteCmd.OpsCenterURL = g.APIKeyDeleteCmd.Flag("ops-url", "remote OpsCenter URL").Required().String()

	// get cluster diagnostics report
	g.ReportCmd.CmdClause = g.Command("report", "Generate cluster diagnostics report")
	g.ReportCmd.FilePath = g.ReportCmd.Flag("file", "target report file name").Default("report.tar.gz").String()

	// operations on sites
	g.SiteCmd.CmdClause = g.Command("site", "operations on gravity sites")

	// list sites
	g.SiteListCmd.CmdClause = g.SiteCmd.Command("list", "list sites").Hidden()
	g.SiteListCmd.OpsCenterURL = g.SiteListCmd.Flag("ops-url", "remote OpsCenter URL").String()

	// start
	g.SiteStartCmd.CmdClause = g.SiteCmd.Command("start", "start site controller (runs inside cluster)").Hidden()
	g.SiteStartCmd.ConfigPath = g.SiteStartCmd.Arg("config", "path to a configuration directory").String()
	g.SiteStartCmd.InitPath = g.SiteStartCmd.Flag("init-from", "path to init packages").String()

	// init
	g.SiteInitCmd.CmdClause = g.SiteCmd.Command("init", "import site state from external database").Hidden()
	g.SiteInitCmd.ConfigPath = g.SiteInitCmd.Arg("config", "path to configuration directory").String()
	g.SiteInitCmd.InitPath = g.SiteInitCmd.Flag("init-from", "path to import state directory").String()

	// status
	g.SiteStatusCmd.CmdClause = g.SiteCmd.Command("status", "check system status").Hidden()

	// info
	g.SiteInfoCmd.CmdClause = g.SiteCmd.Command("info", "Prints local cluster information to the console").Hidden()
	g.SiteInfoCmd.Format = common.Format(g.SiteInfoCmd.Flag("output", "Output format, supported formats: json, text"))

	// complete install step
	g.SiteCompleteCmd.CmdClause = g.SiteCmd.Command("complete", "Marks the final install step as completed").Hidden()
	g.SiteCompleteCmd.Support = g.SiteCompleteCmd.Flag("support", "set remote support status: 'on' or 'off'").Default("on").String()

	// password reset for local gravity site user
	g.SiteResetPasswordCmd.CmdClause = g.SiteCmd.Command("reset-password", "reset password for local user").Hidden()

	// local site
	g.LocalSiteCmd.CmdClause = g.Command("local-site", "Prints the local cluster domain name to the console").Hidden()

	// RPC agent
	g.RPCAgentCmd.CmdClause = g.Command("agent", "RPC agent")

	g.RPCAgentDeployCmd.CmdClause = g.RPCAgentCmd.Command("deploy", "deploy RPC agents across cluster nodes, and run specified execution function").Hidden()
	g.RPCAgentDeployCmd.Args = g.RPCAgentDeployCmd.Arg("arg", "additional arguments").Strings()

	g.RPCAgentShutdownCmd.CmdClause = g.RPCAgentCmd.Command("shutdown", "request agents to shut down").Hidden()

	g.RPCAgentInstallCmd.CmdClause = g.RPCAgentCmd.Command("install", "install and launch local RPC agent service").Hidden()
	g.RPCAgentInstallCmd.Args = g.RPCAgentInstallCmd.Arg("arg", "additional arguments").Strings()

	g.RPCAgentRunCmd.CmdClause = g.RPCAgentCmd.Command("run", "run RPC agent").Hidden()
	g.RPCAgentRunCmd.Args = g.RPCAgentRunCmd.Arg("arg", "additional arguments").Strings()

	g.SystemCmd.CmdClause = g.Command("system", "operations on system components")

	g.SystemRotateCertsCmd.CmdClause = g.SystemCmd.Command("rotate-certs", "Renew cluster certificates on a node").Hidden()
	g.SystemRotateCertsCmd.ClusterName = g.SystemRotateCertsCmd.Arg("cluster-name", "Name of the local cluster").Required().String()
	g.SystemRotateCertsCmd.ValidFor = g.SystemRotateCertsCmd.Flag("valid-for", "Validity duration in Go format").Default("26280h").Duration()
	g.SystemRotateCertsCmd.CAPath = g.SystemRotateCertsCmd.Flag("ca-path", "Use previously exported CA file instead of package").String()

	g.SystemExportCACmd.CmdClause = g.SystemCmd.Command("export-ca", "Export cluster CA, must be run on a master node").Hidden()
	g.SystemExportCACmd.ClusterName = g.SystemExportCACmd.Arg("cluster-name", "Name of the local cluster").Required().String()
	g.SystemExportCACmd.CAPath = g.SystemExportCACmd.Arg("path", "File path to export CA at").Required().String()

	g.SystemUninstallCmd.CmdClause = g.SystemCmd.Command("uninstall", "uninstall gravity from the host").Hidden()
	g.SystemUninstallCmd.Confirmed = g.SystemUninstallCmd.Flag("confirm", "confirm uninstall").Bool()

	g.SystemPullUpdatesCmd.CmdClause = g.SystemCmd.Command("pull-updates", "Pull new package updates from the system").Hidden()
	g.SystemPullUpdatesCmd.OpsCenterURL = g.SystemPullUpdatesCmd.Flag("ops-url", "remote OpsCenter URL").String()
	g.SystemPullUpdatesCmd.RuntimePackage = Locator(g.SystemPullUpdatesCmd.Flag("runtime-package", "The name of the runtime package to update to").Required())

	g.SystemUpdateCmd.CmdClause = g.SystemCmd.Command("update", "Update this system by installing newer version of system packages").Hidden()
	g.SystemUpdateCmd.ChangesetID = g.SystemUpdateCmd.Flag("changeset-id", "Assign ID to this update operation (will be autogenerated if missing)").String()
	g.SystemUpdateCmd.ServiceName = g.SystemUpdateCmd.Flag("service-name", "The name of the service to run update as a systemd unit").String()
	g.SystemUpdateCmd.WithStatus = g.SystemUpdateCmd.Flag("with-status", "Verify the system status at the end of the operation").Bool()
	g.SystemUpdateCmd.RuntimePackage = Locator(g.SystemUpdateCmd.Flag("runtime-package", "The name of the runtime package to update to").Required())

	g.SystemReinstallCmd.CmdClause = g.SystemCmd.Command("reinstall", "reinstall package on the system").Hidden()
	g.SystemReinstallCmd.Package = Locator(g.SystemReinstallCmd.Arg("pkg", "the package to generate unit file for").Required())
	g.SystemReinstallCmd.ServiceName = g.SystemReinstallCmd.Flag("service-name", "optional service name to run operation from systemd unit").String()
	g.SystemReinstallCmd.Labels = configure.KeyValParam(g.SystemReinstallCmd.Flag("labels", "labels to describe the package"))

	g.SystemHistoryCmd.CmdClause = g.SystemCmd.Command("history", "list system update history").Hidden()

	// ask the current active master to step down
	g.SystemStepDownCmd.CmdClause = g.SystemCmd.Command("step-down", "Ask the active master to step down").Hidden()

	g.SystemRollbackCmd.CmdClause = g.SystemCmd.Command("rollback", "starts rollback").Hidden()
	g.SystemRollbackCmd.ChangesetID = g.SystemRollbackCmd.Flag("changeset-id", "optionally select changeset id to rollback to").String()
	g.SystemRollbackCmd.ServiceName = g.SystemRollbackCmd.Flag("service-name", "setting service name starts upgrade as a system service instead of foreground process").String()
	g.SystemRollbackCmd.WithStatus = g.SystemRollbackCmd.Flag("with-status", "Verify the system status at the end of the operation").Bool()

	// system services
	g.SystemServiceCmd.CmdClause = g.SystemCmd.Command("service", "operations on system services")

	// install a new system service
	g.SystemServiceInstallCmd.CmdClause = g.SystemServiceCmd.Command("install", "install a new service").Hidden()
	g.SystemServiceInstallCmd.Package = Locator(g.SystemServiceInstallCmd.Arg("pkg", "the package to generate unit file for").Required())
	g.SystemServiceInstallCmd.ConfigPackage = Locator(g.SystemServiceInstallCmd.Arg("conf-pkg", "the configuration package used to launch the service with").Required())
	g.SystemServiceInstallCmd.StartCommand = g.SystemServiceInstallCmd.Flag("start-command", "the command used to start the service").Required().String()
	g.SystemServiceInstallCmd.StartPreCommand = g.SystemServiceInstallCmd.Flag("start-pre-command", "command executed before the start command").String()
	g.SystemServiceInstallCmd.StartPostCommand = g.SystemServiceInstallCmd.Flag("start-post-command", "command executed after the start command").String()
	g.SystemServiceInstallCmd.StopCommand = g.SystemServiceInstallCmd.Flag("stop-command", "the command used to stop the service").String()
	g.SystemServiceInstallCmd.StopPostCommand = g.SystemServiceInstallCmd.Flag("stop-post-command", "the command executed after the stop command").String()
	g.SystemServiceInstallCmd.Timeout = g.SystemServiceInstallCmd.Flag("timeout", "the number of seconds to wait for the service to start up before consider it failed").Default("0").Int()
	g.SystemServiceInstallCmd.Type = g.SystemServiceInstallCmd.Flag("type", "the type of the service").String()
	g.SystemServiceInstallCmd.Restart = g.SystemServiceInstallCmd.Flag("restart", "service restart policy").Default("always").String()
	g.SystemServiceInstallCmd.LimitNoFile = g.SystemServiceInstallCmd.Flag("limit-nofile", "ulimit for number of open files").Int()
	g.SystemServiceInstallCmd.KillMode = g.SystemServiceInstallCmd.Flag("kill-mode", "kill mode is a systemd KillMode setting").Default("none").String()

	// uninstall system service
	g.SystemServiceUninstallCmd.CmdClause = g.SystemServiceCmd.Command("uninstall", "uninstall service, supply either package or service name").Hidden()
	g.SystemServiceUninstallCmd.Package = Locator(g.SystemServiceUninstallCmd.Flag("package", "the package related to this service"))
	g.SystemServiceUninstallCmd.Name = g.SystemServiceUninstallCmd.Flag("name", "the service name").String()

	// check status of a service
	g.SystemServiceStatusCmd.CmdClause = g.SystemServiceCmd.Command("status", "status of a package service, supply either package or service name ").Hidden()
	g.SystemServiceStatusCmd.Package = Locator(g.SystemServiceStatusCmd.Flag("package", "the package related to this service"))
	g.SystemServiceStatusCmd.Name = g.SystemServiceStatusCmd.Flag("name", "service name to check").String()

	// list running services
	g.SystemServiceListCmd.CmdClause = g.SystemServiceCmd.Command("list", "list running services").Hidden()

	g.SystemReportCmd.CmdClause = g.SystemCmd.Command("report", "collect system diagnostics and output as gzipped tarball to terminal").Hidden()
	g.SystemReportCmd.Filter = g.SystemReportCmd.Flag("filter", "collect only specific diagnostics ('system', 'kubernetes'). Collect everything if unspecified").Strings()
	g.SystemReportCmd.Compressed = g.SystemReportCmd.Flag("compressed", "whether to compress the tarball").Default("true").Bool()

	g.SystemStateDirCmd.CmdClause = g.SystemCmd.Command("state-dir", "show where all gravity data is stored on the node").Hidden()

	// manage docker devicemapper environment
	g.SystemDevicemapperCmd.CmdClause = g.SystemCmd.Command("devicemapper", "manage docker devicemapper environment").Hidden()
	g.SystemDevicemapperMountCmd.CmdClause = g.SystemDevicemapperCmd.Command("mount", "configure devicemapper environment").Hidden()
	g.SystemDevicemapperMountCmd.Disk = g.SystemDevicemapperMountCmd.Arg("disk", "disk/partition to use for physical volume").String()
	g.SystemDevicemapperUnmountCmd.CmdClause = g.SystemDevicemapperCmd.Command("unmount", "remove devicemapper environment").Hidden()
	g.SystemDevicemapperSystemDirCmd.CmdClause = g.SystemDevicemapperCmd.Command("system-dir", "query the location of the lvm system directory").Hidden()

	// network helpers
	g.SystemEnablePromiscModeCmd.CmdClause = g.SystemCmd.Command("enable-promisc-mode", "Put network interface into promiscuous mode").Hidden()
	g.SystemEnablePromiscModeCmd.Iface = g.SystemEnablePromiscModeCmd.Arg("name", "Name of the interface (i.e. docker0)").Default(defaults.DockerBridge).String()

	g.SystemDisablePromiscModeCmd.CmdClause = g.SystemCmd.Command("disable-promisc-mode", "Remove promiscuous mode flag and deduplication rules").Hidden()
	g.SystemDisablePromiscModeCmd.Iface = g.SystemDisablePromiscModeCmd.Arg("name", "Name of the interface (i.e. docker0)").Default(defaults.DockerBridge).String()

	g.SystemExportRuntimeJournalCmd.CmdClause = g.SystemCmd.Command("export-runtime-journal", "Export runtime journal logs to a file").Hidden()
	g.SystemExportRuntimeJournalCmd.OutputFile = g.SystemExportRuntimeJournalCmd.Flag("output", "Name of resulting tarball. Output to stdout if unspecified").String()

	g.SystemStreamRuntimeJournalCmd.CmdClause = g.SystemCmd.Command("stream-runtime-journal", "Stream runtime journal to stdout").Hidden()

	// pruning cluster resources
	g.GarbageCollectCmd.CmdClause = g.Command("gc", "Prune cluster resources")
	g.GarbageCollectCmd.Phase = g.GarbageCollectCmd.Flag("phase", "Specific phase to execute").String()
	g.GarbageCollectCmd.PhaseTimeout = g.GarbageCollectCmd.Flag("timeout", "Phase execution timeout").
		Default(defaults.PhaseTimeout).
		Hidden().
		Duration()
	g.GarbageCollectCmd.Resume = g.GarbageCollectCmd.Flag("resume", "Resume aborted operation").Bool()
	g.GarbageCollectCmd.Manual = g.GarbageCollectCmd.Flag("manual", "Do not start the operation automatically").Short('m').Bool()
	g.GarbageCollectCmd.Confirmed = g.GarbageCollectCmd.Flag("confirm", "Confirm to remove unrelated docker images").Short('c').Bool()
	g.GarbageCollectCmd.Force = g.GarbageCollectCmd.Flag("force", "Force phase execution").Bool()

	// system clean up tasks
	systemGCCmd := g.SystemCmd.Command("gc", "Run system clean up tasks")

	// clean up stale journal files
	g.SystemGCJournalCmd.CmdClause = systemGCCmd.Command("journal",
		"Clean up stale journal directories. "+
			"Directories that do not match the effective systemd machine-id will be removed.").Hidden()
	g.SystemGCJournalCmd.MachineIDFile = g.SystemGCJournalCmd.Flag("machine-id-from",
		fmt.Sprintf("Optional file path to read effective systemd machine-id from. "+
			"If unspecified, %v will be used to read the id. ",
			defaults.SystemdMachineIDFile)).String()
	g.SystemGCJournalCmd.LogDir = g.SystemGCJournalCmd.Flag("log-dir", "Location of the journal files").Default(defaults.SystemdLogDir).String()

	g.SystemGCPackageCmd.CmdClause = systemGCCmd.Command("package", "Prune unused packages.")
	g.SystemGCPackageCmd.DryRun = g.SystemGCPackageCmd.Flag("dry-run", "Only list packages to remove w/o removing them").Bool()
	g.SystemGCPackageCmd.Cluster = g.SystemGCPackageCmd.Flag("cluster", "Whether to prune cluster packages").Bool()

	g.SystemGCRegistryCmd.CmdClause = systemGCCmd.Command("registry", "Prune unused docker images on this node.")
	g.SystemGCRegistryCmd.Confirm = g.SystemGCRegistryCmd.Flag("confirm", "Confirm to remove unrelated docker").Bool()
	g.SystemGCRegistryCmd.DryRun = g.SystemGCRegistryCmd.Flag("dry-run", "Only list docker images to remove w/o removing them").Bool()

	// operations on planet (planet plugin)
	g.PlanetCmd.CmdClause = g.Command("planet", "operations with planet").Hidden()

	g.PlanetEnterCmd.CmdClause = g.PlanetCmd.Command("enter", "enters currently installed planet").Hidden()

	g.PlanetStatusCmd.CmdClause = g.PlanetCmd.Command("status", "calls status for currently installed planet").Hidden()

	g.EnterCmd.CmdClause = g.Command("enter", "enter planet").Hidden()
	g.EnterCmd.Args = g.EnterCmd.Arg("arg", "additional arguments to the container").Strings()

	g.ExecCmd.CmdClause = g.Command("exec", "Run command in a planet container").Interspersed(false)
	g.ExecCmd.TTY = g.ExecCmd.Flag("tty", "Allocate a pseudo-TTY").Short('t').Bool()
	g.ExecCmd.Stdin = g.ExecCmd.Flag("interactive", "Keep stdin open").Short('i').Bool()
	g.ExecCmd.Cmd = g.ExecCmd.Arg("command", "Command to execute").Required().String()
	g.ExecCmd.Args = g.ExecCmd.Arg("arg", "Additional arguments to command").Strings()

	g.ShellCmd.CmdClause = g.Command("shell", "Start an interactive shell in a planet container")

	// resource management
	g.ResourceCmd.CmdClause = g.Command("resource", "Management of configuration resources")

	// create one or many resources
	g.ResourceCreateCmd.CmdClause = g.ResourceCmd.Command("create", fmt.Sprintf("Create or update a configuration resource, e.g. gravity resource create oidc.yaml. Supported resources are: %v", modules.Get().SupportedResources()))
	g.ResourceCreateCmd.Filename = g.ResourceCreateCmd.Arg("filename", "resource definition file").String()
	g.ResourceCreateCmd.Upsert = g.ResourceCreateCmd.Flag("force", "Overwrites a resource if it already exists. (update)").Short('f').Bool()
	g.ResourceCreateCmd.User = g.ResourceCreateCmd.Flag("user", "user to create resource for, defaults to currently logged in user").String()

	// remove one or many resources
	g.ResourceRemoveCmd.CmdClause = g.ResourceCmd.Command("rm", fmt.Sprintf("Remove a configuration resource, e.g. gravity resource rm oidc google. Supported resources are: %v", modules.Get().SupportedResourcesToRemove()))
	g.ResourceRemoveCmd.Kind = g.ResourceRemoveCmd.Arg("kind", fmt.Sprintf("resource kind, one of %v", modules.Get().SupportedResourcesToRemove())).Required().String()
	g.ResourceRemoveCmd.Name = g.ResourceRemoveCmd.Arg("name", "resource name, e.g. github").Required().String()
	g.ResourceRemoveCmd.Force = g.ResourceRemoveCmd.Flag("force", "Do not return errors if a resource is not found").Short('f').Bool()
	g.ResourceRemoveCmd.User = g.ResourceRemoveCmd.Flag("user", "user to remove resource for, defaults to currently logged in user").String()

	// get resources returns resources
	g.ResourceGetCmd.CmdClause = g.ResourceCmd.Command("get", fmt.Sprintf("Get configuration resources, e.g. gravity get oidc. Supported resources are: %v", modules.Get().SupportedResources()))
	g.ResourceGetCmd.Kind = g.ResourceGetCmd.Arg("kind", fmt.Sprintf("resource kind, one of %v", modules.Get().SupportedResources())).Required().String()
	g.ResourceGetCmd.Name = g.ResourceGetCmd.Arg("name", fmt.Sprintf("optional resource name, lists all resources if omitted")).String()
	g.ResourceGetCmd.Format = common.Format(g.ResourceGetCmd.Flag("format", "resource format, e.g. 'text', 'json' or 'yaml'").Default(string(constants.EncodingText)))
	g.ResourceGetCmd.WithSecrets = g.ResourceGetCmd.Flag("with-secrets", "include secret properties like private keys").Default("false").Bool()
	g.ResourceGetCmd.User = g.ResourceGetCmd.Flag("user", "user to display resources for, defaults to currently logged in user").String()

	return g
}

// Locator defines a command line flag that accepts input
// in package locator format
func Locator(s kingpin.Settings) *loc.Locator {
	l := new(loc.Locator)
	s.SetValue(l)
	return l
}

// DockerStorageDriver defines a command line flag that recognizes
// Docker storage drivers
func DockerStorageDriver(s kingpin.Settings, allowed []string) *dockerStorageDriver {
	driver := &dockerStorageDriver{allowed: allowed}
	s.SetValue(driver)
	return driver
}

// Set validates value as a Docker storage driver
func (r *dockerStorageDriver) Set(value string) error {
	if !utils.StringInSlice(r.allowed, value) {
		return trace.BadParameter("unrecognized docker storage driver %q, supported are: %v",
			value, r.allowed)
	}
	r.value = value
	return nil
}

// String returns the value of the storage driver
func (r *dockerStorageDriver) String() string {
	if r == nil {
		return ""
	}
	return r.value
}

// dockerStorageDriver is a string that only accepts recognized
// Docker storage driver name as a value
type dockerStorageDriver struct {
	allowed []string
	value   string
}
