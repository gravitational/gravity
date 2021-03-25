/*
Copyright 2018-2019 Gravitational, Inc.

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
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	appapi "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/systemservice"
	clusterupdate "github.com/gravitational/gravity/lib/update/cluster"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/cli"

	"github.com/gravitational/configure/cstrings"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "cli")

// ConfigureEnvironment updates PATH environment variable to include
// gravity binary search locations
func ConfigureEnvironment() error {
	path := os.Getenv(defaults.PathEnv)
	return trace.Wrap(os.Setenv(defaults.PathEnv, fmt.Sprintf("%v:%v",
		path, defaults.PathEnvVal)))
}

// Run parses CLI arguments and executes an appropriate gravity command
func Run(g *Application) (err error) {
	log.Debugf("Executing: %v.", os.Args)
	err = ConfigureEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	args, extraArgs := cstrings.SplitAt(os.Args[1:], "--")
	cmd, err := g.Parse(args)
	if err != nil {
		return trace.Wrap(err)
	}

	if *g.Debug {
		utils.InitGRPCLoggerWithDefaults()
	} else {
		utils.InitGRPCLoggerFromEnvironment()
	}

	if *g.UID != -1 || *g.GID != -1 {
		return SwitchPrivileges(*g.UID, *g.GID)
	}
	err = InitAndCheck(g, cmd)
	if err != nil {
		return trace.Wrap(err)
	}

	execer := CmdExecer{
		Exe:       getExec(g, cmd, extraArgs),
		Parser:    cli.ArgsParserFunc(parseArgs),
		Args:      args,
		ExtraArgs: extraArgs,
	}
	return execer.Execute()
}

// InitAndCheck initializes the CLI application according to the provided
// flags and checks that the command is being executed in an appropriate
// environmnent
func InitAndCheck(g *Application, cmd string) error {
	trace.SetDebug(*g.Debug)
	level := logrus.InfoLevel
	if *g.Debug {
		level = logrus.DebugLevel
	}
	systemLogSet := true
	if *g.SystemLogFile == "" {
		systemLogSet = false
		*g.SystemLogFile = defaults.GravitySystemLogPath
	}
	switch cmd {
	case g.SiteStartCmd.FullCommand():
		teleutils.InitLogger(teleutils.LoggingForDaemon, level)
	case g.RPCAgentDeployCmd.FullCommand(),
		g.RPCAgentInstallCmd.FullCommand(),
		g.RPCAgentRunCmd.FullCommand(),
		g.PlanCmd.FullCommand(),
		g.PlanDisplayCmd.FullCommand(),
		g.UpgradeCmd.FullCommand(),
		g.StartCmd.FullCommand(),
		g.StopCmd.FullCommand(),
		g.RollbackCmd.FullCommand(),
		g.ResourceCreateCmd.FullCommand():
		if *g.Debug {
			teleutils.InitLogger(teleutils.LoggingForDaemon, level)
		}
	default:
		teleutils.InitLogger(teleutils.LoggingForCLI, level)
	}
	logrus.SetFormatter(&trace.TextFormatter{})

	// the following commands write logs to the system log file (in
	// addition to journald)
	switch cmd {
	case g.InstallCmd.FullCommand(),
		g.WizardCmd.FullCommand(),
		g.JoinCmd.FullCommand(),
		g.AutoJoinCmd.FullCommand(),
		g.StartCmd.FullCommand(),
		g.UpdateTriggerCmd.FullCommand(),
		g.UpdatePlanInitCmd.FullCommand(),
		g.UpgradeCmd.FullCommand(),
		g.UpdateUploadCmd.FullCommand(),
		g.RPCAgentRunCmd.FullCommand(),
		g.LeaveCmd.FullCommand(),
		g.RemoveCmd.FullCommand(),
		g.ResumeCmd.FullCommand(),
		g.PlanResumeCmd.FullCommand(),
		g.PlanExecuteCmd.FullCommand(),
		g.PlanRollbackCmd.FullCommand(),
		g.RollbackCmd.FullCommand(),
		g.ResourceCreateCmd.FullCommand(),
		g.ResourceRemoveCmd.FullCommand(),
		g.OpsAgentCmd.FullCommand():
		utils.InitLogging(level, *g.SystemLogFile)
		// several command also duplicate their logs to the file in
		// the current directory for convenience, unless the user set their
		// own location
		switch cmd {
		case g.InstallCmd.FullCommand(), g.JoinCmd.FullCommand(), g.StartCmd.FullCommand():
			if *g.SystemLogFile == defaults.GravitySystemLogPath {
				utils.InitLogging(level, defaults.GravitySystemLogFile)
			}
		}
	default:
		if systemLogSet {
			// For all commands, use the system log file explicitly set on command line
			utils.InitLogging(level, *g.SystemLogFile)
		}
	}
	log.WithField("args", os.Args).Info("Start.")

	if *g.ProfileEndpoint != "" {
		err := process.StartProfiling(context.TODO(), *g.ProfileEndpoint, *g.ProfileTo)
		if err != nil {
			log.WithError(err).Warn("Failed to setup profiling.")
		}
	}

	utils.DetectPlanetEnvironment()

	// the following commands must be run inside deployed cluster
	switch cmd {
	case g.UpdateCompleteCmd.FullCommand(),
		g.UpdateTriggerCmd.FullCommand(),
		g.RemoveCmd.FullCommand():
		if err := checkRunningInGravity(g); err != nil {
			return trace.Wrap(err)
		}
	}

	// the following commands must be run as root
	switch cmd {
	case g.SystemUpdateCmd.FullCommand(),
		g.UpgradeCmd.FullCommand(),
		g.RollbackCmd.FullCommand(),
		g.SystemRollbackCmd.FullCommand(),
		g.StopCmd.FullCommand(),
		g.StartCmd.FullCommand(),
		g.SystemUninstallCmd.FullCommand(),
		g.UpdateSystemCmd.FullCommand(),
		g.RPCAgentShutdownCmd.FullCommand(),
		g.RPCAgentInstallCmd.FullCommand(),
		g.RPCAgentRunCmd.FullCommand(),
		g.SystemServiceInstallCmd.FullCommand(),
		g.SystemServiceUninstallCmd.FullCommand(),
		g.EnterCmd.FullCommand(),
		g.PlanetEnterCmd.FullCommand(),
		g.UpdatePlanInitCmd.FullCommand(),
		g.ResumeCmd.FullCommand(),
		g.PlanCmd.FullCommand(),
		g.PlanDisplayCmd.FullCommand(),
		g.PlanExecuteCmd.FullCommand(),
		g.PlanRollbackCmd.FullCommand(),
		g.PlanResumeCmd.FullCommand(),
		g.PlanCompleteCmd.FullCommand(),
		g.InstallCmd.FullCommand(),
		g.JoinCmd.FullCommand(),
		g.AutoJoinCmd.FullCommand(),
		g.LeaveCmd.FullCommand(),
		g.RemoveCmd.FullCommand(),
		g.BackupCmd.FullCommand(),
		g.RestoreCmd.FullCommand(),
		g.GarbageCollectCmd.FullCommand(),
		g.SystemGCRegistryCmd.FullCommand(),
		g.OpsAgentCmd.FullCommand(),
		g.CheckCmd.FullCommand(),
		g.ReportCmd.FullCommand():
		if err := checkRunningAsRoot(); err != nil {
			return trace.Wrap(err)
		}
	}

	// following commands must be run outside the planet container
	switch cmd {
	case g.SystemUpdateCmd.FullCommand(),
		g.SystemRollbackCmd.FullCommand(),
		g.UpdateSystemCmd.FullCommand(),
		g.UpgradeCmd.FullCommand(),
		g.RollbackCmd.FullCommand(),
		g.SystemGCRegistryCmd.FullCommand(),
		g.SystemUninstallCmd.FullCommand(),
		g.PlanetEnterCmd.FullCommand(),
		g.ResumeCmd.FullCommand(),
		g.PlanExecuteCmd.FullCommand(),
		g.PlanRollbackCmd.FullCommand(),
		g.PlanResumeCmd.FullCommand(),
		g.LeaveCmd.FullCommand(),
		g.EnterCmd.FullCommand():
		if utils.CheckInPlanet() {
			return trace.BadParameter("this command must be run outside of planet container")
		}
	}

	// following commands must be run inside the planet container
	switch cmd {
	case g.SystemGCJournalCmd.FullCommand():
		if !utils.CheckInPlanet() {
			return trace.BadParameter("this command must be run inside planet container")
		}
	}

	return nil
}

// getExec returns the Executable function to execute the specified gravity cmd.
func getExec(g *Application, cmd string, extraArgs []string) Executable {
	return func() error {
		return Execute(g, cmd, extraArgs)
	}
}

// Execute executes the gravity command given with cmd
func Execute(g *Application, cmd string, extraArgs []string) (err error) {
	switch cmd {
	case g.VersionCmd.FullCommand():
		return printVersion(*g.VersionCmd.Output)
	case g.SiteStartCmd.FullCommand():
		return startSite(*g.SiteStartCmd.ConfigPath, *g.SiteStartCmd.InitPath)
	case g.SiteInitCmd.FullCommand():
		return initCluster(*g.SiteInitCmd.ConfigPath, *g.SiteInitCmd.InitPath)
	case g.SiteStatusCmd.FullCommand():
		return statusSite()
	}

	var localEnv *localenv.LocalEnvironment
	switch cmd {
	case g.InstallCmd.FullCommand(), g.JoinCmd.FullCommand():
		if *g.StateDir != "" {
			if err := state.SetStateDir(*g.StateDir); err != nil {
				return trace.Wrap(err)
			}
		}
		localEnv, err = g.NewInstallEnv()
		if err != nil {
			return trace.Wrap(err)
		}
		defer localEnv.Close()
	case g.UpdateUploadCmd.FullCommand():
		localStateDir, err := localenv.LocalGravityDir()
		if err != nil {
			return trace.Wrap(err)
		}
		localEnv, err = localenv.New(localStateDir)
		if err != nil {
			return trace.Wrap(err)
		}
		defer localEnv.Close()
	default:
		localEnv, err = g.NewLocalEnv()
		if err != nil {
			return trace.Wrap(err)
		}
		defer localEnv.Close()
	}

	// the following commands must run when Kubernetes is available (can
	// be inside gravity cluster or generic Kubernetes cluster)
	switch cmd {
	case g.AppInstallCmd.FullCommand(),
		g.AppListCmd.FullCommand(),
		g.AppUpgradeCmd.FullCommand(),
		g.AppRollbackCmd.FullCommand(),
		g.AppUninstallCmd.FullCommand(),
		g.AppHistoryCmd.FullCommand():
		if err := httplib.InGravity(localEnv.DNS.Addr()); err != nil {
			if !httplib.InKubernetes() {
				return trace.BadParameter("this command must be executed " +
					"inside a Kubernetes cluster")
			}
		}
	}

	switch cmd {
	case g.OpsAgentCmd.FullCommand():
		return agent(localEnv, agentConfig{
			systemLogFile: *g.SystemLogFile,
			userLogFile:   *g.UserLogFile,
			serviceName:   *g.OpsAgentCmd.ServiceName,
			packageAddr:   *g.OpsAgentCmd.PackageAddr,
			advertiseAddr: g.OpsAgentCmd.AdvertiseAddr.String(),
			serverAddr:    *g.OpsAgentCmd.ServerAddr,
			token:         *g.OpsAgentCmd.Token,
			vars:          *g.OpsAgentCmd.Vars,
			serviceUID:    *g.OpsAgentCmd.ServiceUID,
			serviceGID:    *g.OpsAgentCmd.ServiceGID,
			cloudProvider: *g.OpsAgentCmd.CloudProvider,
		})
	case g.WizardCmd.FullCommand():
		config, err := NewWizardConfig(localEnv, g)
		if err != nil {
			return trace.Wrap(err)
		}
		return startInstall(localEnv, *config)
	case g.InstallCmd.FullCommand():
		config, err := NewInstallConfig(localEnv, g)
		if err != nil {
			return trace.Wrap(err)
		}
		return startInstall(localEnv, *config)
	case g.StopCmd.FullCommand():
		return stopGravity(localEnv,
			*g.StopCmd.Confirmed)
	case g.StartCmd.FullCommand():
		// If advertise address was explicitly provided to the start command,
		// launch the reconfigure operation.
		if *g.StartCmd.AdvertiseAddr != "" {
			config, err := newReconfigureConfig(localEnv, g)
			if err != nil {
				return trace.Wrap(err)
			}
			return reconfigureCluster(localEnv, *config,
				*g.StartCmd.Confirmed)
		}
		return startGravity(localEnv,
			*g.StartCmd.Confirmed)
	case g.JoinCmd.FullCommand():
		return join(localEnv, g, NewJoinConfig(g))
	case g.AutoJoinCmd.FullCommand():
		return autojoin(localEnv, g, autojoinConfig{
			systemLogFile: *g.SystemLogFile,
			userLogFile:   *g.UserLogFile,
			clusterName:   *g.AutoJoinCmd.ClusterName,
			role:          *g.AutoJoinCmd.Role,
			systemDevice:  *g.AutoJoinCmd.SystemDevice,
			mounts:        *g.AutoJoinCmd.Mounts,
			fromService:   *g.AutoJoinCmd.FromService,
			serviceURL:    *g.AutoJoinCmd.ServiceAddr,
			token:         *g.AutoJoinCmd.Token,
			advertiseAddr: *g.AutoJoinCmd.AdvertiseAddr,
		})
	case g.UpdateCheckCmd.FullCommand():
		return updateCheck(localEnv, *g.UpdateCheckCmd.App)
	case g.UpdateTriggerCmd.FullCommand():
		updateEnv, err := g.NewUpdateEnv()
		if err != nil {
			return trace.Wrap(err)
		}
		defer updateEnv.Close()
		return updateTrigger(localEnv, updateEnv, upgradeConfig{
			upgradePackage:   *g.UpdateTriggerCmd.App,
			manual:           *g.UpdateTriggerCmd.Manual,
			skipVersionCheck: *g.UpdateTriggerCmd.SkipVersionCheck,
			force:            *g.UpdateTriggerCmd.Force,
			userConfig: clusterupdate.UserConfig{
				SkipWorkers:     *g.UpdateTriggerCmd.SkipWorkers,
				ParallelWorkers: *g.UpdateTriggerCmd.ParallelWorkers,
			},
		})
	case g.UpdatePlanInitCmd.FullCommand():
		updateEnv, err := g.NewUpdateEnv()
		if err != nil {
			return trace.Wrap(err)
		}
		defer updateEnv.Close()
		return initUpdateOperationPlan(localEnv, updateEnv, clusterupdate.UserConfig{
			SkipWorkers:     *g.UpdatePlanInitCmd.SkipWorkers,
			ParallelWorkers: *g.UpdatePlanInitCmd.ParallelWorkers,
		})
	case g.UpgradeCmd.FullCommand():
		updateEnv, err := g.NewUpdateEnv()
		if err != nil {
			return trace.Wrap(err)
		}
		defer updateEnv.Close()
		if *g.UpgradeCmd.Resume {
			return executeUpdatePhase(localEnv, g, PhaseParams{
				PhaseID:          fsm.RootPhase,
				Timeout:          *g.UpgradeCmd.Timeout,
				SkipVersionCheck: *g.UpgradeCmd.SkipVersionCheck,
				Block:            *g.UpgradeCmd.Block,
			})
		}
		if *g.UpgradeCmd.Phase != "" {
			return executeUpdatePhase(localEnv, g, PhaseParams{
				PhaseID:          *g.UpgradeCmd.Phase,
				Force:            *g.UpgradeCmd.Force,
				Timeout:          *g.UpgradeCmd.Timeout,
				SkipVersionCheck: *g.UpgradeCmd.SkipVersionCheck,
				Block:            true, // Direct phase executions run in foreground.
			})
		}
		config, err := newUpgradeConfig(g)
		if err != nil {
			return trace.Wrap(err)
		}
		return updateTrigger(localEnv, updateEnv, *config)
	case g.ResumeCmd.FullCommand():
		return resumeOperation(localEnv, g,
			PhaseParams{
				Force:            *g.ResumeCmd.Force,
				Timeout:          *g.ResumeCmd.PhaseTimeout,
				SkipVersionCheck: *g.ResumeCmd.SkipVersionCheck,
				OperationID:      *g.ResumeCmd.OperationID,
			})
	case g.PlanExecuteCmd.FullCommand():
		return executePhase(localEnv, g,
			PhaseParams{
				PhaseID:          *g.PlanExecuteCmd.Phase,
				Force:            *g.PlanExecuteCmd.Force,
				Timeout:          *g.PlanExecuteCmd.PhaseTimeout,
				SkipVersionCheck: *g.PlanCmd.SkipVersionCheck,
				OperationID:      *g.PlanCmd.OperationID,
				Block:            true, // Direct phase executions run in foreground.
			})
	case g.PlanSetCmd.FullCommand():
		return setPhase(localEnv, g, SetPhaseParams{
			OperationID: *g.PlanCmd.OperationID,
			PhaseID:     *g.PlanSetCmd.Phase,
			State:       *g.PlanSetCmd.State,
		})
	case g.PlanResumeCmd.FullCommand():
		return resumeOperation(localEnv, g,
			PhaseParams{
				Force:            *g.PlanResumeCmd.Force,
				Timeout:          *g.PlanResumeCmd.PhaseTimeout,
				SkipVersionCheck: *g.PlanCmd.SkipVersionCheck,
				OperationID:      *g.PlanCmd.OperationID,
				Block:            *g.PlanResumeCmd.Block,
			})
	case g.PlanRollbackCmd.FullCommand():
		return rollbackPhase(localEnv, g,
			PhaseParams{
				PhaseID:          *g.PlanRollbackCmd.Phase,
				Force:            *g.PlanRollbackCmd.Force,
				Timeout:          *g.PlanRollbackCmd.PhaseTimeout,
				SkipVersionCheck: *g.PlanCmd.SkipVersionCheck,
				OperationID:      *g.PlanCmd.OperationID,
			})
	case g.PlanDisplayCmd.FullCommand():
		outputFormat := *g.PlanDisplayCmd.Output
		if *g.PlanDisplayCmd.Short {
			outputFormat = constants.EncodingShort
		}
		return displayOperationPlan(localEnv, g,
			*g.PlanCmd.OperationID, displayPlanOptions{
				format: outputFormat,
				follow: *g.PlanDisplayCmd.Follow,
			})
	case g.PlanCompleteCmd.FullCommand():
		return completeOperationPlan(localEnv, g, *g.PlanCmd.OperationID)
	case g.RollbackCmd.FullCommand():
		return rollbackPlan(localEnv, g,
			PhaseParams{
				Timeout:          *g.RollbackCmd.PhaseTimeout,
				SkipVersionCheck: *g.RollbackCmd.SkipVersionCheck,
				OperationID:      *g.RollbackCmd.OperationID,
				DryRun:           *g.RollbackCmd.DryRun,
			}, *g.RollbackCmd.Confirmed)
	case g.LeaveCmd.FullCommand():
		return leave(localEnv, leaveConfig{
			force:     *g.LeaveCmd.Force,
			confirmed: *g.LeaveCmd.Confirm,
		})
	case g.RemoveCmd.FullCommand():
		return remove(localEnv, removeConfig{
			server:    *g.RemoveCmd.Node,
			force:     *g.RemoveCmd.Force,
			confirmed: *g.RemoveCmd.Confirm,
		})
	case g.StatusClusterCmd.FullCommand():
		printOptions := printOptions{
			token:       *g.StatusClusterCmd.Token,
			operationID: *g.StatusClusterCmd.OperationID,
			quiet:       *g.Silent,
			format:      *g.StatusClusterCmd.Output,
		}
		if *g.StatusClusterCmd.Tail {
			return tailStatus(localEnv, *g.StatusClusterCmd.OperationID)
		}
		if *g.StatusClusterCmd.Seconds != 0 {
			return statusPeriodic(localEnv, printOptions, *g.StatusClusterCmd.Seconds)
		} else {
			return status(localEnv, printOptions)
		}
	case g.StatusHistoryCmd.FullCommand():
		return statusHistory()
	case g.UpdateUploadCmd.FullCommand():
		tarballEnv, err := getTarballEnvironForUpgrade(localEnv, *g.StateDir)
		if err != nil {
			return trace.Wrap(err)
		}
		return uploadUpdate(context.Background(), tarballEnv, localEnv,
			*g.UpdateUploadCmd.OpsCenterURL)
	case g.AppPackageCmd.FullCommand():
		return appPackage(localEnv)
		// app commands
	case g.AppInstallCmd.FullCommand():
		return releaseInstall(localEnv, releaseInstallConfig{
			Image:     *g.AppInstallCmd.Image,
			Release:   *g.AppInstallCmd.Name,
			Namespace: *g.AppCmd.Namespace,
			valuesConfig: valuesConfig{
				Values: *g.AppInstallCmd.Set,
				Files:  *g.AppInstallCmd.Values,
			},
			registryConfig: registryConfig{
				Registry: *g.AppInstallCmd.Registry,
				CAPath:   *g.AppInstallCmd.RegistryCA,
				CertPath: *g.AppInstallCmd.RegistryCert,
				KeyPath:  *g.AppInstallCmd.RegistryKey,
				Username: *g.AppInstallCmd.RegistryUsername,
				Password: *g.AppInstallCmd.RegistryPassword,
				Prefix:   *g.AppInstallCmd.RegistryPrefix,
				Insecure: *g.Insecure,
			},
		})
	case g.AppListCmd.FullCommand():
		return releaseList(localEnv, releaseListConfig{
			Namespace: *g.AppCmd.Namespace,
			All:       *g.AppListCmd.All,
		})
	case g.AppUpgradeCmd.FullCommand():
		return releaseUpgrade(localEnv, releaseUpgradeConfig{
			Namespace: *g.AppCmd.Namespace,
			Release:   *g.AppUpgradeCmd.Release,
			Image:     *g.AppUpgradeCmd.Image,
			valuesConfig: valuesConfig{
				Values: *g.AppUpgradeCmd.Set,
				Files:  *g.AppUpgradeCmd.Values,
			},
			registryConfig: registryConfig{
				Registry: *g.AppUpgradeCmd.Registry,
				CAPath:   *g.AppUpgradeCmd.RegistryCA,
				CertPath: *g.AppUpgradeCmd.RegistryCert,
				KeyPath:  *g.AppUpgradeCmd.RegistryKey,
				Username: *g.AppInstallCmd.RegistryUsername,
				Password: *g.AppInstallCmd.RegistryPassword,
				Prefix:   *g.AppInstallCmd.RegistryPrefix,
				Insecure: *g.Insecure,
			},
		})
	case g.AppRollbackCmd.FullCommand():
		return releaseRollback(localEnv, releaseRollbackConfig{
			Namespace: *g.AppCmd.Namespace,
			Release:   *g.AppRollbackCmd.Release,
			Revision:  *g.AppRollbackCmd.Revision,
		})
	case g.AppUninstallCmd.FullCommand():
		return releaseUninstall(localEnv, releaseUninstallConfig{
			Namespace: *g.AppCmd.Namespace,
			Release:   *g.AppUninstallCmd.Release,
		})
	case g.AppHistoryCmd.FullCommand():
		return releaseHistory(localEnv, releaseHistoryConfig{
			Namespace: *g.AppCmd.Namespace,
			Release:   *g.AppHistoryCmd.Release,
		})
	case g.AppSyncCmd.FullCommand():
		return appSync(localEnv, appSyncConfig{
			Image: *g.AppSyncCmd.Image,
			registryConfig: registryConfig{
				Registry:           *g.AppSyncCmd.Registry,
				CAPath:             *g.AppSyncCmd.RegistryCA,
				CertPath:           *g.AppSyncCmd.RegistryCert,
				KeyPath:            *g.AppSyncCmd.RegistryKey,
				Username:           *g.AppSyncCmd.RegistryUsername,
				Password:           *g.AppSyncCmd.RegistryPassword,
				Prefix:             *g.AppSyncCmd.RegistryPrefix,
				Insecure:           *g.Insecure,
				ScanningRepository: g.AppSyncCmd.ScanningRepository,
				ScanningTagPrefix:  g.AppSyncCmd.ScanningTagPrefix,
			},
		})
	case g.AppSearchCmd.FullCommand():
		return appSearch(localEnv,
			*g.AppSearchCmd.Pattern,
			*g.AppSearchCmd.Remote,
			*g.AppSearchCmd.All)
	case g.AppRebuildIndexCmd.FullCommand():
		return appRebuildIndex(localEnv)
	case g.AppIndexCmd.FullCommand():
		return appIndex(localEnv,
			*g.AppIndexCmd.MergeInto)
		// internal (hidden) app commands
	case g.AppImportCmd.FullCommand():
		if len(*g.AppImportCmd.SetImages) != 0 || len(*g.AppImportCmd.SetDeps) != 0 || *g.AppImportCmd.Version != "" {
			if !*g.AppImportCmd.Vendor {
				fmt.Printf("found one of --set-image, --set-dep or --version flags: turning on --vendor mode\n")
				*g.AppImportCmd.Vendor = true
			}
		}
		if *g.AppImportCmd.Vendor && *g.AppImportCmd.RegistryURL == "" {
			return trace.BadParameter("vendoring mode requires --registry-url")
		}
		req := &appapi.ImportRequest{
			Repository:             *g.AppImportCmd.Repository,
			PackageName:            *g.AppImportCmd.Name,
			PackageVersion:         *g.AppImportCmd.Version,
			Vendor:                 *g.AppImportCmd.Vendor,
			Force:                  *g.AppImportCmd.Force,
			ExcludePatterns:        *g.AppImportCmd.Excludes,
			IncludePaths:           *g.AppImportCmd.IncludePaths,
			ResourcePatterns:       *g.AppImportCmd.VendorPatterns,
			IgnoreResourcePatterns: *g.AppImportCmd.VendorIgnorePatterns,
			SetImages:              *g.AppImportCmd.SetImages,
			SetDeps:                *g.AppImportCmd.SetDeps,
		}
		return importApp(localEnv,
			*g.AppImportCmd.RegistryURL,
			*g.AppImportCmd.DockerURL,
			*g.AppImportCmd.Source,
			req,
			*g.AppImportCmd.OpsCenterURL,
			*g.Silent,
			*g.AppImportCmd.Parallel)
	case g.AppExportCmd.FullCommand():
		return exportApp(localEnv,
			*g.AppExportCmd.Locator,
			*g.AppExportCmd.OpsCenterURL,
			*g.AppExportCmd.RegistryURL)
	case g.AppDeleteCmd.FullCommand():
		return deleteApp(localEnv,
			*g.AppDeleteCmd.Locator,
			*g.AppDeleteCmd.OpsCenterURL,
			*g.AppDeleteCmd.Force)
	case g.AppPackageListCmd.FullCommand():
		return listApps(localEnv,
			*g.AppPackageListCmd.Repository,
			*g.AppPackageListCmd.Type,
			*g.AppPackageListCmd.ShowHidden,
			*g.AppPackageListCmd.OpsCenterURL)
	case g.AppStatusCmd.FullCommand():
		return statusApp(localEnv,
			*g.AppStatusCmd.Locator,
			*g.AppStatusCmd.OpsCenterURL)
	case g.AppPackageUninstallCmd.FullCommand():
		return uninstallAppPackage(localEnv,
			*g.AppPackageUninstallCmd.Locator)
	case g.AppPullCmd.FullCommand():
		return pullApp(localEnv,
			*g.AppPullCmd.Package,
			*g.AppPullCmd.OpsCenterURL,
			*g.AppPullCmd.Labels,
			*g.AppPullCmd.Force)
	case g.AppPushCmd.FullCommand():
		return pushApp(localEnv,
			*g.AppPushCmd.Package,
			*g.AppPushCmd.OpsCenterURL)
	case g.AppHookCmd.FullCommand():
		req := appapi.HookRunRequest{
			Application: *g.AppHookCmd.Package,
			Hook:        schema.HookType(*g.AppHookCmd.HookName),
			Env:         *g.AppHookCmd.Env,
		}
		return outputAppHook(localEnv, req)
	case g.AppUnpackCmd.FullCommand():
		return unpackAppResources(localEnv,
			*g.AppUnpackCmd.Package,
			*g.AppUnpackCmd.Dir,
			*g.AppUnpackCmd.OpsCenterURL,
			*g.AppUnpackCmd.ServiceUID)
	// package commands
	case g.PackImportCmd.FullCommand():
		return importPackage(localEnv,
			*g.PackImportCmd.Path,
			*g.PackImportCmd.Locator,
			*g.PackImportCmd.CheckManifest,
			*g.PackImportCmd.OpsCenterURL,
			*g.PackImportCmd.Labels)
	case g.PackUnpackCmd.FullCommand():
		return unpackPackage(localEnv,
			*g.PackUnpackCmd.Locator,
			*g.PackUnpackCmd.Dir,
			*g.PackUnpackCmd.OpsCenterURL,
			nil)
	case g.PackExportCmd.FullCommand():
		mode, err := strconv.ParseUint(*g.PackExportCmd.FileMask, 8, 32)
		if err != nil {
			return trace.BadParameter("invalid file access mask %v: %v", *g.PackExportCmd.FileMask, err)
		}
		return exportPackage(localEnv,
			*g.PackExportCmd.Locator,
			*g.PackExportCmd.OpsCenterURL,
			*g.PackExportCmd.File,
			os.FileMode(mode),
			*g.PackExportCmd.FileLabel)
	case g.PackListCmd.FullCommand():
		return listPackages(localEnv,
			*g.PackListCmd.Repository,
			*g.PackListCmd.OpsCenterURL)
	case g.PackDeleteCmd.FullCommand():
		return deletePackage(localEnv,
			*g.PackDeleteCmd.Locator,
			*g.PackDeleteCmd.Force,
			*g.PackDeleteCmd.OpsCenterURL)
	case g.PackConfigureCmd.FullCommand():
		return configurePackage(localEnv,
			*g.PackConfigureCmd.Package,
			*g.PackConfigureCmd.ConfPackage,
			*g.PackConfigureCmd.Args)
	case g.PackCommandCmd.FullCommand():
		return executePackageCommand(localEnv,
			*g.PackCommandCmd.Command,
			*g.PackCommandCmd.Package,
			g.PackCommandCmd.ConfPackage,
			*g.PackCommandCmd.Args)
	case g.PackPushCmd.FullCommand():
		return pushPackage(localEnv,
			*g.PackPushCmd.Package,
			*g.PackPushCmd.OpsCenterURL)
	case g.PackPullCmd.FullCommand():
		return pullPackage(localEnv,
			*g.PackPullCmd.Package,
			*g.PackPullCmd.OpsCenterURL,
			*g.PackPullCmd.Labels,
			*g.PackPullCmd.Force)
	case g.PackLabelsCmd.FullCommand():
		return updatePackageLabels(localEnv,
			*g.PackLabelsCmd.Package,
			*g.PackLabelsCmd.OpsCenterURL,
			*g.PackLabelsCmd.Add,
			*g.PackLabelsCmd.Remove)
		// OpsCenter commands
	case g.OpsConnectCmd.FullCommand():
		return connectToOpsCenter(localEnv,
			*g.OpsConnectCmd.OpsCenterURL,
			*g.OpsConnectCmd.Username,
			*g.OpsConnectCmd.Password)
	case g.OpsDisconnectCmd.FullCommand():
		return disconnectFromOpsCenter(localEnv,
			*g.OpsDisconnectCmd.OpsCenterURL)
	case g.OpsListCmd.FullCommand():
		return listOpsCenters(localEnv)
	case g.UserCreateCmd.FullCommand():
		return createUser(localEnv,
			*g.UserCreateCmd.OpsCenterURL,
			*g.UserCreateCmd.Email,
			*g.UserCreateCmd.Type,
			*g.UserCreateCmd.Password)
	case g.UserDeleteCmd.FullCommand():
		return deleteUser(localEnv,
			*g.UserDeleteCmd.OpsCenterURL,
			*g.UserDeleteCmd.Email)
	case g.APIKeyCreateCmd.FullCommand():
		return createAPIKey(localEnv,
			*g.APIKeyCreateCmd.OpsCenterURL,
			*g.APIKeyCreateCmd.Email)
	case g.APIKeyListCmd.FullCommand():
		return getAPIKeys(localEnv,
			*g.APIKeyListCmd.OpsCenterURL,
			*g.APIKeyListCmd.Email)
	case g.APIKeyDeleteCmd.FullCommand():
		return deleteAPIKey(localEnv,
			*g.APIKeyDeleteCmd.OpsCenterURL,
			*g.APIKeyDeleteCmd.Email,
			*g.APIKeyDeleteCmd.Token)
	case g.ReportCmd.FullCommand():
		return getClusterReport(localEnv,
			*g.ReportCmd.FilePath,
			*g.ReportCmd.Since)
	// cluster commands
	case g.SiteListCmd.FullCommand():
		return listSites(localEnv, *g.SiteListCmd.OpsCenterURL)
	case g.SiteInfoCmd.FullCommand():
		return printLocalClusterInfo(localEnv,
			*g.SiteInfoCmd.Format)
	case g.SiteCompleteCmd.FullCommand():
		return completeInstallerStep(localEnv,
			*g.SiteCompleteCmd.Support)
	case g.SiteResetPasswordCmd.FullCommand():
		return resetPassword(localEnv)
	case g.StatusResetCmd.FullCommand():
		return resetClusterState(localEnv,
			*g.StatusResetCmd.Confirmed)
	case g.RegistryListCmd.FullCommand():
		return listRegistryContents(context.Background(), localEnv, registryConnectionRequest{
			address:  *g.RegistryListCmd.Registry,
			caPath:   *g.RegistryListCmd.CAPath,
			certPath: *g.RegistryListCmd.CertPath,
			keyPath:  *g.RegistryListCmd.KeyPath,
		}, *g.RegistryListCmd.Format)
	case g.LocalSiteCmd.FullCommand():
		return getLocalSite(localEnv)
	// system service commands
	case g.SystemRotateCertsCmd.FullCommand():
		return rotateCertificates(localEnv, rotateOptions{
			clusterName: *g.SystemRotateCertsCmd.ClusterName,
			validFor:    *g.SystemRotateCertsCmd.ValidFor,
			caPath:      *g.SystemRotateCertsCmd.CAPath,
		})
	case g.SystemExportCACmd.FullCommand():
		return exportCertificateAuthority(localEnv,
			*g.SystemExportCACmd.ClusterName,
			*g.SystemExportCACmd.CAPath)
	case g.SystemTeleportShowConfigCmd.FullCommand():
		return showTeleportConfig(localEnv,
			*g.SystemTeleportShowConfigCmd.Package)
	case g.SystemReinstallCmd.FullCommand():
		return systemReinstall(localEnv,
			*g.SystemReinstallCmd.Package,
			*g.SystemReinstallCmd.ServiceName,
			*g.SystemReinstallCmd.Labels,
			*g.SystemReinstallCmd.ClusterRole)
	case g.SystemHistoryCmd.FullCommand():
		return systemHistory(localEnv)
	case g.SystemClusterInfoCmd.FullCommand():
		return systemClusterInfo(localEnv)
	case g.SystemPullUpdatesCmd.FullCommand():
		return systemPullUpdates(localEnv,
			*g.SystemPullUpdatesCmd.OpsCenterURL,
			*g.SystemPullUpdatesCmd.RuntimePackage)
	case g.SystemUpdateCmd.FullCommand():
		return systemUpdate(localEnv,
			*g.SystemUpdateCmd.ChangesetID,
			*g.SystemUpdateCmd.ServiceName,
			*g.SystemUpdateCmd.WithStatus,
			*g.SystemUpdateCmd.RuntimePackage)
	case g.UpdateSystemCmd.FullCommand():
		return systemUpdate(localEnv,
			*g.UpdateSystemCmd.ChangesetID,
			*g.UpdateSystemCmd.ServiceName,
			*g.UpdateSystemCmd.WithStatus,
			*g.UpdateSystemCmd.RuntimePackage)
	case g.SystemRollbackCmd.FullCommand():
		return systemRollback(localEnv,
			*g.SystemRollbackCmd.ChangesetID,
			*g.SystemRollbackCmd.ServiceName,
			*g.SystemRollbackCmd.WithStatus)
	case g.SystemStepDownCmd.FullCommand():
		return stepDown(localEnv)
	case g.BackupCmd.FullCommand():
		return backup(localEnv,
			*g.BackupCmd.Tarball,
			*g.BackupCmd.Timeout,
			*g.BackupCmd.Follow,
			*g.Silent)
	case g.RestoreCmd.FullCommand():
		return restore(localEnv,
			*g.RestoreCmd.Tarball,
			*g.RestoreCmd.Timeout,
			*g.RestoreCmd.Follow,
			*g.Silent)
	case g.SystemServiceInstallCmd.FullCommand():
		req := &systemservice.NewPackageServiceRequest{
			Package:       *g.SystemServiceInstallCmd.Package,
			ConfigPackage: *g.SystemServiceInstallCmd.ConfigPackage,
			ServiceSpec: systemservice.ServiceSpec{
				StartCommand:     *g.SystemServiceInstallCmd.StartCommand,
				StartPreCommands: []string{*g.SystemServiceInstallCmd.StartPreCommand},
				StartPostCommand: *g.SystemServiceInstallCmd.StartPostCommand,
				StopCommand:      *g.SystemServiceInstallCmd.StopCommand,
				StopPostCommand:  *g.SystemServiceInstallCmd.StopPostCommand,
				Timeout:          *g.SystemServiceInstallCmd.Timeout,
				Type:             *g.SystemServiceInstallCmd.Type,
				LimitNoFile:      *g.SystemServiceInstallCmd.LimitNoFile,
				Restart:          *g.SystemServiceInstallCmd.Restart,
				KillMode:         *g.SystemServiceInstallCmd.KillMode,
			},
		}
		return systemServiceInstall(localEnv, req)
	case g.SystemServiceUninstallCmd.FullCommand():
		return systemServiceUninstall(localEnv,
			*g.SystemServiceUninstallCmd.Package,
			*g.SystemServiceUninstallCmd.Name)
	case g.SystemServiceListCmd.FullCommand():
		return systemServiceList(localEnv)
	case g.SystemServiceStartCmd.FullCommand():
		return systemServiceStart(localEnv, *g.SystemServiceStartCmd.Package)
	case g.SystemServiceStopCmd.FullCommand():
		return systemServiceStop(localEnv, *g.SystemServiceStopCmd.Package)
	case g.SystemServiceJournalCmd.FullCommand():
		return systemServiceJournal(localEnv,
			*g.SystemServiceJournalCmd.Package,
			*g.SystemServiceJournalCmd.Args)
	case g.SystemServiceStatusCmd.FullCommand():
		return systemServiceStatus(localEnv,
			*g.SystemServiceStatusCmd.Package)
	case g.SystemUninstallCmd.FullCommand():
		return systemUninstall(localEnv, *g.SystemUninstallCmd.Confirmed)
	case g.SystemReportCmd.FullCommand():
		return systemReport(localEnv,
			*g.SystemReportCmd.Filter,
			*g.SystemReportCmd.Compressed,
			*g.SystemReportCmd.Output,
			*g.SystemReportCmd.Since)
	case g.SystemStateDirCmd.FullCommand():
		return printStateDir()
	case g.SystemExportRuntimeJournalCmd.FullCommand():
		return exportRuntimeJournal(localEnv,
			*g.SystemExportRuntimeJournalCmd.OutputFile,
			*g.SystemExportRuntimeJournalCmd.Since,
			*g.SystemExportRuntimeJournalCmd.Export)
	case g.SystemStreamRuntimeJournalCmd.FullCommand():
		return streamRuntimeJournal(localEnv,
			*g.SystemStreamRuntimeJournalCmd.Since,
			*g.SystemStreamRuntimeJournalCmd.Export)
	case g.SystemSelinuxBootstrapCmd.FullCommand():
		return bootstrapSELinux(localEnv,
			*g.SystemSelinuxBootstrapCmd.Path,
			*g.StateDir,
			*g.SystemSelinuxBootstrapCmd.VxlanPort)
	case g.GarbageCollectCmd.FullCommand():
		return garbageCollect(localEnv, *g.GarbageCollectCmd.Manual, *g.GarbageCollectCmd.Confirmed)
	case g.SystemGCJournalCmd.FullCommand():
		return removeUnusedJournalFiles(localEnv,
			*g.SystemGCJournalCmd.MachineIDFile,
			*g.SystemGCJournalCmd.LogDir)
	case g.SystemGCPackageCmd.FullCommand():
		return removeUnusedPackages(localEnv,
			*g.SystemGCPackageCmd.DryRun,
			*g.SystemGCPackageCmd.Cluster)
	case g.SystemGCRegistryCmd.FullCommand():
		return removeUnusedImages(localEnv,
			*g.SystemGCRegistryCmd.DryRun,
			*g.SystemGCRegistryCmd.Confirm)
	case g.PlanetEnterCmd.FullCommand(), g.EnterCmd.FullCommand():
		return planetEnter(localEnv, extraArgs)
	case g.ExecCmd.FullCommand():
		return planetExec(localEnv,
			*g.ExecCmd.TTY,
			*g.ExecCmd.Stdin,
			*g.ExecCmd.Cmd,
			*g.ExecCmd.Args)
	case g.ShellCmd.FullCommand():
		return planetShell(localEnv)
	case g.PlanetStatusCmd.FullCommand():
		return getPlanetStatus(localEnv, extraArgs)
	case g.UsersInviteCmd.FullCommand():
		return inviteUser(localEnv,
			*g.UsersInviteCmd.Name,
			*g.UsersInviteCmd.Roles,
			*g.UsersInviteCmd.TTL)
	case g.UsersResetCmd.FullCommand():
		return resetUser(localEnv,
			*g.UsersResetCmd.Name,
			*g.UsersResetCmd.TTL)
	case g.ResourceCreateCmd.FullCommand():
		return createResource(localEnv, g,
			*g.ResourceCreateCmd.Filename,
			*g.ResourceCreateCmd.Upsert,
			*g.ResourceCreateCmd.User,
			*g.ResourceCreateCmd.Manual,
			*g.ResourceCreateCmd.Confirmed)
	case g.ResourceRemoveCmd.FullCommand():
		return removeResource(localEnv, g,
			*g.ResourceRemoveCmd.Kind,
			*g.ResourceRemoveCmd.Name,
			*g.ResourceRemoveCmd.Force,
			*g.ResourceRemoveCmd.User,
			*g.ResourceRemoveCmd.Manual,
			*g.ResourceRemoveCmd.Confirmed)
	case g.ResourceGetCmd.FullCommand():
		return getResources(localEnv,
			*g.ResourceGetCmd.Kind,
			*g.ResourceGetCmd.Name,
			*g.ResourceGetCmd.WithSecrets,
			*g.ResourceGetCmd.Format,
			*g.ResourceGetCmd.User)
	case g.RPCAgentDeployCmd.FullCommand():
		return rpcAgentDeploy(localEnv,
			deployOptions{
				leaderArgs: *g.RPCAgentDeployCmd.LeaderArgs,
				nodeArgs:   *g.RPCAgentDeployCmd.NodeArgs,
				version:    *g.RPCAgentDeployCmd.Version,
			})
	case g.RPCAgentInstallCmd.FullCommand():
		return rpcAgentInstall(localEnv, *g.RPCAgentInstallCmd.Args)
	case g.RPCAgentRunCmd.FullCommand():
		updateEnv, err := g.NewUpdateEnv()
		if err != nil {
			return trace.Wrap(err)
		}
		defer updateEnv.Close()
		return rpcAgentRun(localEnv, updateEnv,
			*g.RPCAgentRunCmd.Args)
	case g.RPCAgentStatusCmd.FullCommand():
		return rpcAgentStatus(localEnv)
	case g.RPCAgentShutdownCmd.FullCommand():
		return rpcAgentShutdown(localEnv)
	case g.CheckCmd.FullCommand():
		return executePreflightChecks(localEnv, preflightChecksConfig{
			manifestPath: *g.CheckCmd.ManifestFile,
			imagePath:    *g.CheckCmd.ImagePath,
			profileName:  *g.CheckCmd.Profile,
			autoFix:      *g.CheckCmd.AutoFix,
			timeout:      *g.CheckCmd.Timeout,
		})
	case g.TopCmd.FullCommand():
		return top(localEnv,
			*g.TopCmd.Interval,
			*g.TopCmd.Step)
	}
	return trace.NotFound("unknown command %v", cmd)
}

// SwitchPrivileges switches user privileges and executes
// the same command but with different user id and group id
func SwitchPrivileges(uid, gid int) error {
	// see this for details: https://github.com/golang/go/issues/1435
	// setuid is broken, so we can't use it
	fullPath, err := exec.LookPath(os.Args[0])
	if err != nil {
		return trace.Wrap(err)
	}
	cred := &syscall.Credential{}
	if uid != -1 {
		cred.Uid = uint32(uid)
	}
	if gid != -1 {
		cred.Gid = uint32(gid)
	}
	args := cstrings.WithoutFlag(os.Args, "--uid")
	args = cstrings.WithoutFlag(args, "--gid")
	cmd := exec.Cmd{
		Path: fullPath,
		Args: args,
		SysProcAttr: &syscall.SysProcAttr{
			Credential: cred,
		},
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return cmd.Run()
}

// pickSiteHost detects the currently active master host.
// It does this by probing master IP from /etc/container-environment and localhost.
func pickSiteHost() (string, error) {
	var hosts []string
	// master IP takes priority, as it contains IP of the k8s API server.
	// this is a temporary hack, need to figure out the proper way
	if f, err := os.Open(defaults.ContainerEnvironmentFile); err == nil {
		defer f.Close()
		r := bufio.NewReader(f)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				break
			}
			parts := strings.Split(line, "=")
			if len(parts) == 2 && parts[0] == "KUBE_APISERVER" {
				targetHost := strings.Trim(strings.TrimSpace(parts[1]), `"`)
				log.Infof("found apiserver: %v", targetHost)
				return targetHost, nil
			}
		}
	}
	hosts = append(hosts, "127.0.0.1")
	log.Infof("trying these hosts: %v", hosts)
	for _, host := range hosts {
		log.Infof("connecting to %s", host)
		r, err := http.Get(fmt.Sprintf("http://%s:8080", host))
		if err == nil && r != nil {
			log.Infof(r.Status)
			return host, nil
		} else if err != nil {
			log.Infof(err.Error())
		}
	}
	return "", trace.Errorf("failed to find a gravity site to connect to")
}

func checkRunningInGravity(environ LocalEnvironmentFactory) error {
	env, err := environ.NewLocalEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer env.Close()
	err = httplib.InGravity(env.DNS.Addr())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func checkRunningAsRoot() error {
	if os.Geteuid() != 0 {
		return trace.BadParameter("this command should be run as root")
	}
	return nil
}
