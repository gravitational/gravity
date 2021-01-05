package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/tele/cli"

	"github.com/gravitational/trace"
)

func pull(env *localenv.LocalEnvironment, app, outFile string, force, quiet bool) error {
	opsURL, err := env.SelectOpsCenterWithDefault("", defaults.DistributionOpsCenter)
	if err != nil {
		return trace.Wrap(err)
	}

	operator, err := env.OperatorService(opsURL)
	if err != nil {
		return trace.Wrap(err)
	}

	packages, err := env.PackageService(opsURL)
	if err != nil {
		return trace.Wrap(err)
	}

	locator, err := cli.MakeLocator(app)
	if err != nil {
		return trace.Wrap(err)
	}
	name := locator.Name

	// tele ls displays base images as "gravity" while the actual image
	// name is "telekube" (for legacy reasons); same for "opscenter"
	// (legacy name) and "hub" (new name)
	switch locator.Name {
	case constants.BaseImageName:
		locator.Name = constants.LegacyBaseImageName
	case constants.HubImageName:
		locator.Name = constants.LegacyHubImageName
	}

	progress := cli.NewProgress(context.TODO(), "Download", quiet)
	defer progress.Stop()

	if opsURL == modules.Get().TeleRepository() {
		progress.NextStep("Not logged in. Using default Gravitational Hub")
	} else {
		progress.NextStep("Using Hub %v", opsURL)
	}

	if locator.Version == loc.LatestVersion {
		locator, err = pack.FindLatestPackage(packages, *locator)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if outFile == "" {
		outFile = fmt.Sprintf("%v-%v.tar", name, locator.Version)
	}

	fi, err := utils.StatFile(outFile)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if fi != nil && !force {
		return trace.AlreadyExists("file %v already exists, provide --force flag to overwrite it", outFile)
	}

	progress.NextStep("Requesting cluster image from %v", opsURL)

	reader, err := operator.GetAppInstaller(ops.AppInstallerRequest{
		AccountID:   defaults.SystemAccountID,
		Application: *locator,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	f, err := os.Create(outFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	progress.NextStep(fmt.Sprintf("Downloading %v:%v", name, locator.Version))

	_, err = io.Copy(f, reader)
	if err != nil {
		return trace.Wrap(err)
	}

	progress.NextStep(fmt.Sprintf("Application %v downloaded", app))

	return nil
}
