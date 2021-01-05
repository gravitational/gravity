package cli

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/localenv/credentials"
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
		hub := *tele.Hub
		if hub == "" {
			hub = *tele.BuildCmd.Repository
			if hub != "" {
				common.PrintWarn("Flag --repository is obsolete " +
					"and will be removed in future version, " +
					"please use --hub instead.")
			}
		}
		credentials, err := tele.cliCredentials()
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		return buildClusterImage(context.Background(), buildParameters{
			BuildParameters: cli.BuildParameters{
				StateDir:         *tele.StateDir,
				SourcePath:       *tele.BuildCmd.Path,
				OutPath:          *tele.BuildCmd.OutFile,
				Overwrite:        *tele.BuildCmd.Overwrite,
				SkipVersionCheck: *tele.BuildCmd.SkipVersionCheck,
				Silent:           *tele.BuildCmd.Quiet,
				Verbose:          *tele.BuildCmd.Verbose,
				BaseImage:        *tele.BuildCmd.BaseImage,
				Insecure:         *tele.Insecure,
				Vendor: service.VendorRequest{
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
				},
			},
			RemoteSupportAddress: *tele.BuildCmd.RemoteSupport,
			RemoteSupportToken:   *tele.BuildCmd.RemoteSupportToken,
			CACertPath:           *tele.BuildCmd.CACert,
			EncryptionKey:        *tele.BuildCmd.EncryptionKey,
			Credentials:          credentials,
		})
	case tele.LoginCmd.FullCommand():
		opsCenter := *tele.Hub
		if opsCenter == "" {
			opsCenter = *tele.LoginCmd.OpsCenter
			if opsCenter != "" {
				common.PrintWarn("Flag --ops/-o is obsolete " +
					"and will be removed in future version, " +
					"please use --hub instead.")
			}
		}
		common.PrintWarn(teleLoginWarning)
		return login(loginConfig{
			stateDir:    *tele.StateDir,
			insecure:    *tele.Insecure,
			opsCenter:   opsCenter,
			siteDomain:  *tele.LoginCmd.Cluster,
			connectorID: *tele.LoginCmd.ConnectorID,
			ttl:         *tele.LoginCmd.TTL,
			apiKey:      *tele.Token,
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
	}

	env, err := tele.makeLocalEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	defer env.Close()

	switch cmd {
	case tele.PushCmd.FullCommand():
		return push(env,
			*tele.PushCmd.Tarball,
			*tele.PushCmd.Force,
			*tele.PushCmd.Quiet)
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

// makeLocalEnvironment returns an instance of the local environment.
func (t *Application) makeLocalEnvironment() (*localenv.LocalEnvironment, error) {
	// If --state-dir was provided on the CLI, it is used as a Gravity local
	// key store, otherwise default location (~/.gravity/config) is used.
	keystoreDir := *t.StateDir
	stateDir, closer, err := t.stateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credentials, err := t.cliCredentials()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	env, err := localenv.NewLocalEnvironment(
		localenv.LocalEnvironmentArgs{
			StateDir:         stateDir,
			LocalKeyStoreDir: keystoreDir,
			Insecure:         *t.Insecure,
			Credentials:      credentials,
			Close:            closer,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return env, nil
}

// stateDir returns state directory for the local environment and an optional
// cleanup function that removes this directory in case it is a temporary one.
func (t *Application) stateDir() (dir string, closer func() error, err error) {
	if *t.StateDir == "" {
		return tempStateDir()
	}
	return *t.StateDir, nil, nil
}

// tempStateDir creates temporary state directory and returns its name and
// the cleanup function that removes it.
func tempStateDir() (dir string, closer func() error, err error) {
	dir, err = ioutil.TempDir("", "tele")
	if err != nil {
		return "", nil, trace.ConvertSystemError(err)
	}
	return dir, func() error {
		return os.RemoveAll(dir)
	}, nil
}

// cliCredentials returns credentials set on the CLI, if any, or nil.
func (t *Application) cliCredentials() (*credentials.Credentials, error) {
	if *t.Token != "" && *t.Hub == "" {
		return nil, trace.BadParameter("--hub flag must be provided if --token flag is provided")
	}
	if *t.Hub != "" {
		return credentials.FromTokenAndHub(*t.Token, *t.Hub), nil
	}
	return nil, trace.NotFound("no CLI credentials provided")
}

const teleLoginWarning = `The "tele login" command is obsolete and will be removed in a future version.

Please use the "tsh login" command to log into the cluster using an interactive user:

$ tsh login --proxy=<auth-gateway-addr>

To use a non-interactive agent user token with "tele", provide it directly to the command:

$ tele pull <image> --token=<auth-token> --hub=<hub-addr>

See https://gravitational.com/gravity/docs/access/ for more information.
`
