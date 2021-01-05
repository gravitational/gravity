package cli

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/tool/common"
	"github.com/gravitational/gravity/tool/tele/cli"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "cli")

// Run parses CLI arguments and executes an appropriate tele command
func Run(tele Application) error {
	log.Debugf("Executing: %v.", os.Args)
	cmd, err := tele.Parse(os.Args[1:])
	if err != nil {
		return trace.Wrap(err)
	}

	trace.SetDebug(*tele.Debug)
	if *tele.Debug {
		teleutils.InitLogger(teleutils.LoggingForDaemon, logrus.DebugLevel)
	} else {
		teleutils.InitLogger(teleutils.LoggingForCLI, logrus.InfoLevel)
	}

	switch cmd {
	case tele.BuildCmd.FullCommand():
		return build(context.Background(), buildParameters{
			BuildParameters: cli.BuildParameters{
				StateDir:         *tele.StateDir,
				ManifestPath:     *tele.BuildCmd.ManifestPath,
				OutPath:          *tele.BuildCmd.OutFile,
				Overwrite:        *tele.BuildCmd.Overwrite,
				Repository:       *tele.BuildCmd.Repository,
				SkipVersionCheck: *tele.BuildCmd.SkipVersionCheck,
				Silent:           *tele.BuildCmd.Quiet,
				Insecure:         *tele.Insecure,
			},
			RemoteSupportAddress: *tele.BuildCmd.RemoteSupport,
			RemoteSupportToken:   *tele.BuildCmd.RemoteSupportToken,
			CACertPath:           *tele.BuildCmd.CACert,
			EncryptionKey:        *tele.BuildCmd.EncryptionKey,
		}, service.VendorRequest{
			PackageName:            *tele.BuildCmd.Name,
			PackageVersion:         *tele.BuildCmd.Version,
			ResourcePatterns:       *tele.BuildCmd.VendorPatterns,
			IgnoreResourcePatterns: *tele.BuildCmd.VendorIgnorePatterns,
			SetImages:              *tele.BuildCmd.SetImages,
			SetDeps:                *tele.BuildCmd.SetDeps,
			Parallel:               *tele.BuildCmd.Parallel,
			VendorRuntime:          true,
			Helm: helm.RenderParameters{
				Values: *tele.BuildCmd.Values,
				Set:    *tele.BuildCmd.Set,
			},
			Pull: *tele.BuildCmd.Pull,
		})
	case tele.LoginCmd.FullCommand():
		opsCenter := *tele.LoginCmd.Hub
		if opsCenter == "" {
			opsCenter = *tele.LoginCmd.OpsCenter
			if opsCenter != "" {
				common.PrintWarn("Flag --ops/-o is obsolete " +
					"and will be removed in future version, " +
					"please use --hub/-h instead.")
			}
		}
		return login(loginConfig{
			stateDir:    *tele.StateDir,
			insecure:    *tele.Insecure,
			opsCenter:   opsCenter,
			siteDomain:  *tele.LoginCmd.Cluster,
			connectorID: *tele.LoginCmd.ConnectorID,
			ttl:         *tele.LoginCmd.TTL,
			apiKey:      *tele.LoginCmd.Token,
		})
	case tele.StatusCmd.FullCommand():
		return status(loginConfig{
			stateDir: *tele.StateDir,
			insecure: *tele.Insecure,
		})
	case tele.LogoutCmd.FullCommand():
		return logout(context.Background(), loginConfig{
			stateDir: *tele.StateDir,
			insecure: *tele.Insecure,
		})
	case tele.PushCmd.FullCommand():
		return push(
			*tele.PushCmd.Tarball,
			*tele.StateDir,
			*tele.PushCmd.Force,
			*tele.Insecure,
			*tele.PushCmd.Quiet)
	}

	stateDir := *tele.StateDir
	keystoreDir := *tele.StateDir
	if stateDir == "" {
		stateDir, err = ioutil.TempDir("", "tele")
		if err != nil {
			return trace.Wrap(err)
		}
		defer os.RemoveAll(stateDir)
	}

	env, err := localenv.NewLocalEnvironment(
		localenv.LocalEnvironmentArgs{
			StateDir:         stateDir,
			LocalKeyStoreDir: keystoreDir,
			Insecure:         *tele.Insecure,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	defer env.Close()

	switch cmd {
	case tele.PullCmd.FullCommand():
		return pull(env,
			*tele.PullCmd.App,
			*tele.PullCmd.OutFile,
			*tele.PullCmd.Force,
			*tele.PullCmd.Quiet)
	case tele.ListCmd.FullCommand():
		return list(env,
			*tele.ListCmd.All,
			*tele.ListCmd.Format)
	case tele.CreateCmd.FullCommand():
		return createResource(env,
			*tele.CreateCmd.Filename,
			*tele.CreateCmd.Force)
	case tele.GetCmd.FullCommand():
		format := *tele.GetCmd.Output
		if format == "" {
			format = *tele.GetCmd.Format
		}
		return getResources(env,
			*tele.GetCmd.Kind,
			*tele.GetCmd.Name,
			format)
	case tele.RemoveCmd.FullCommand():
		return removeResource(env,
			*tele.RemoveCmd.Kind,
			*tele.RemoveCmd.Name,
			*tele.RemoveCmd.Force)
	}

	// no match among enterprise commands, try open-source
	err = cli.Run(tele.Application)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
