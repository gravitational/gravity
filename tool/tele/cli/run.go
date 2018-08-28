package cli

import (
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/localenv"

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
	case tele.VersionCmd.FullCommand():
		return printVersion(*tele.VersionCmd.Output)
	case tele.BuildCmd.FullCommand():
		buildEnv, err := tele.BuildEnv()
		if err != nil {
			return trace.Wrap(err)
		}
		defer buildEnv.Close()
		return build(BuildParameters{
			BuildEnv:         buildEnv,
			ManifestPath:     *tele.BuildCmd.ManifestPath,
			OutPath:          *tele.BuildCmd.OutFile,
			Overwrite:        *tele.BuildCmd.Overwrite,
			Repository:       *tele.BuildCmd.Repository,
			SkipVersionCheck: *tele.BuildCmd.SkipVersionCheck,
		}, service.VendorRequest{
			PackageName:            *tele.BuildCmd.Name,
			PackageVersion:         *tele.BuildCmd.Version,
			ResourcePatterns:       *tele.BuildCmd.VendorPatterns,
			IgnoreResourcePatterns: *tele.BuildCmd.VendorIgnorePatterns,
			SetImages:              *tele.BuildCmd.SetImages,
			SetDeps:                *tele.BuildCmd.SetDeps,
			Parallel:               *tele.BuildCmd.Parallel,
			VendorRuntime:          true,
		}, *tele.Quiet)
	}

	keystoreDir := *tele.StateDir
	if *tele.StateDir == "" {
		*tele.StateDir, err = ioutil.TempDir("", "tele")
		if err != nil {
			return trace.Wrap(err)
		}
		defer os.RemoveAll(*tele.StateDir)
	}

	env, err := localenv.NewLocalEnvironment(
		localenv.LocalEnvironmentArgs{
			StateDir:         *tele.StateDir,
			LocalKeyStoreDir: keystoreDir,
			Insecure:         *tele.Insecure,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	defer env.Close()

	switch cmd {
	case tele.PullCmd.FullCommand():
		return pull(*env,
			*tele.PullCmd.App,
			*tele.PullCmd.OutFile,
			*tele.PullCmd.Force,
			*tele.Quiet)
	case tele.ListCmd.FullCommand():
		return list(*env,
			*tele.ListCmd.Runtimes,
			*tele.ListCmd.Format)
	}

	return trace.NotFound("unknown command %v", cmd)
}
