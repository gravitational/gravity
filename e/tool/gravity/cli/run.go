package cli

import (
	"os"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	cliutils "github.com/gravitational/gravity/lib/utils/cli"
	"github.com/gravitational/gravity/tool/gravity/cli"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "cli")

func Run(g *Application) error {
	log.WithField("args", os.Args).Debug("Executing command.")
	err := cli.ConfigureEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	args, extraArgs := cstrings.SplitAt(os.Args[1:], "--")
	cmd, err := g.Parse(args)
	if err != nil {
		return trace.Wrap(err)
	}

	if *g.UID != -1 || *g.GID != -1 {
		return cli.SwitchPrivileges(*g.UID, *g.GID)
	}

	err = cli.InitAndCheck(g.Application, cmd)
	if err != nil {
		return trace.Wrap(err)
	}

	execer := cli.CmdExecer{
		Exe:       getExec(g, cmd, extraArgs),
		Parser:    cliutils.ArgsParserFunc(parseArgs),
		Args:      args,
		ExtraArgs: extraArgs,
	}
	return execer.Execute()
}

func getExec(g *Application, cmd string, extraArgs []string) cli.Executable {
	return func() error {
		return execute(g, cmd, extraArgs)
	}
}

func execute(g *Application, cmd string, extraArgs []string) (err error) {
	switch cmd {
	case g.SiteStartCmd.FullCommand():
		return startProcess(
			*g.SiteStartCmd.ConfigPath,
			*g.SiteStartCmd.InitPath)
	}
	// the following enterprise commands require local env
	var localEnv *environment.Local
	switch cmd {
	case g.InstallCmd.FullCommand(),
		g.WizardCmd.FullCommand(),
		g.StatusCmd.FullCommand(),
		g.UpdateDownloadCmd.FullCommand(),
		g.OpsGenerateCmd.FullCommand(),
		g.TunnelEnableCmd.FullCommand(),
		g.TunnelDisableCmd.FullCommand(),
		g.TunnelStatusCmd.FullCommand(),
		g.ResourceCreateCmd.FullCommand(),
		g.ResourceRemoveCmd.FullCommand(),
		g.ResourceGetCmd.FullCommand(),
		g.LicenseInstallCmd.FullCommand(),
		g.LicenseNewCmd.FullCommand(),
		g.LicenseShowCmd.FullCommand(),
		g.SiteInfoCmd.FullCommand():
		ossLocalEnv, err := g.LocalEnv(cmd)
		if err != nil {
			return trace.Wrap(err)
		}
		localEnv = &environment.Local{ossLocalEnv}
		defer localEnv.Close()
	}
	switch cmd {
	case g.InstallCmd.FullCommand():
		if *g.InstallCmd.Resume {
			*g.InstallCmd.Phase = fsm.RootPhase
		}
		if *g.InstallCmd.Phase != "" {
			return executeInstallPhase(localEnv, InstallPhaseParams{
				PhaseParams: cli.PhaseParams{
					PhaseID: *g.InstallCmd.Phase,
					Force:   *g.InstallCmd.Force,
					Timeout: *g.InstallCmd.PhaseTimeout,
				},
				OpsURL:   *g.InstallCmd.OpsCenterURL,
				OpsToken: *g.InstallCmd.OpsCenterToken,
			})
		}
		config := NewInstallConfig(g)
		// if this is an install via an Ops Center, this command needs
		// to determine whether it should be running an installer or a
		// joining agent
		if config.Mode == constants.InstallModeOpsCenter {
			joinEnv, err := g.NewJoinEnv()
			if err != nil {
				return trace.Wrap(err)
			}
			defer joinEnv.Close()
			return installOrJoin(localEnv, &environment.Local{joinEnv}, config)
		}
		return startInstall(localEnv, config)
	case g.WizardCmd.FullCommand():
		return startInstall(localEnv, InstallConfig{
			InstallConfig: cli.InstallConfig{
				Mode:          constants.InstallModeInteractive,
				Insecure:      *g.Insecure,
				UserLogFile:   *g.UserLogFile,
				SystemLogFile: *g.SystemLogFile,
				ServiceUID:    *g.WizardCmd.ServiceUID,
				ServiceGID:    *g.WizardCmd.ServiceGID,
			},
		})
	case g.StatusCmd.FullCommand():
		// only --tunnel flag is specific to the enterprise
		if *g.StatusCmd.Tunnel {
			return remoteAccessStatus(localEnv)
		}
	case g.UpdateDownloadCmd.FullCommand():
		return updateDownload(localEnv, *g.UpdateDownloadCmd.Every)
	case g.OpsGenerateCmd.FullCommand():
		return generateInstaller(localEnv,
			*g.OpsGenerateCmd.Package,
			*g.OpsGenerateCmd.Dir,
			*g.OpsGenerateCmd.CACert,
			*g.OpsGenerateCmd.EncryptionKey,
			*g.OpsGenerateCmd.OpsCenterURL)
	case g.TunnelEnableCmd.FullCommand():
		return updateRemoteAccess(localEnv, true)
	case g.TunnelDisableCmd.FullCommand():
		return updateRemoteAccess(localEnv, false)
	case g.TunnelStatusCmd.FullCommand():
		return remoteAccessStatus(localEnv)
	case g.ResourceCreateCmd.FullCommand():
		return createResource(localEnv, g.Application,
			*g.ResourceCreateCmd.Filename,
			*g.ResourceCreateCmd.Upsert,
			*g.ResourceCreateCmd.User,
			*g.ResourceCreateCmd.Manual,
			*g.ResourceCreateCmd.Confirmed)
	case g.ResourceRemoveCmd.FullCommand():
		return removeResource(localEnv, g.Application,
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
	case g.LicenseInstallCmd.FullCommand():
		return installLicense(localEnv,
			*g.LicenseInstallCmd.Path)
	case g.LicenseNewCmd.FullCommand():
		return newLicense(localEnv,
			*g.LicenseNewCmd.MaxNodes,
			*g.LicenseNewCmd.ValidFor,
			*g.LicenseNewCmd.StopApp,
			*g.LicenseNewCmd.CAKey,
			*g.LicenseNewCmd.CACert,
			*g.LicenseNewCmd.EncryptionKey,
			*g.LicenseNewCmd.CustomerName,
			*g.LicenseNewCmd.CustomerEmail,
			*g.LicenseNewCmd.CustomerMetadata,
			*g.LicenseNewCmd.ProductName,
			*g.LicenseNewCmd.ProductVersion)
	case g.LicenseShowCmd.FullCommand():
		return showLicense(localEnv,
			*g.LicenseShowCmd.Output)
	case g.SiteInfoCmd.FullCommand():
		return printLocalClusterInfo(localEnv,
			*g.SiteInfoCmd.Format)
	}
	// no enterprise commands matched, execute open-source
	return cli.Execute(g.Application, cmd, extraArgs)
}
