package builder

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/utils"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Build builds the standalone application installer using the provided builder
func Build(ctx context.Context, builder *Builder, silent bool) error {
	err := checkBuildEnv()
	if err != nil {
		return trace.Wrap(err)
	}

	locator := builder.Locator()
	progress := utils.NewProgress(ctx, fmt.Sprintf("Build %v:%v", locator.Name,
		locator.Version), 5, silent)
	defer progress.Stop()

	progress.NextStep("Downloading dependencies from %v", builder.Repository)
	err = builder.SyncPackageCache()
	if err != nil {
		return trace.Wrap(err)
	}

	if !builder.SkipVersionCheck {
		err := builder.checkVersion()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	progress.NextStep("Embedding Docker images")
	vendorDir, err := ioutil.TempDir("", "vendor")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(vendorDir)
	stream, err := builder.Vendor(ctx, vendorDir, progress)
	if err != nil {
		return trace.Wrap(err)
	}
	defer stream.Close()

	progress.NextStep("Creating application")
	application, err := builder.CreateApplication(stream)
	if err != nil {
		return trace.Wrap(err)
	}

	progress.NextStep("Generating installer tarball")
	installer, err := builder.GenerateInstaller(*application)
	if err != nil {
		return trace.Wrap(err)
	}
	defer installer.Close()

	progress.NextStep("Writing installer tarball to disk")
	err = builder.WriteInstaller(installer)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// checkBuildEnv makes sure that the environment "tele build" is invoked in is
// suitable, for example, OS is supported and Docker is running
func checkBuildEnv() error {
	if runtime.GOOS != "linux" {
		return trace.BadParameter("tele build is not supported on %v, only "+
			"Linux is supported", runtime.GOOS)
	}
	client, err := docker.NewClient(constants.DockerEngineURL)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = client.Version()
	if err != nil {
		logrus.Error(trace.DebugReport(err))
		return trace.BadParameter("docker is not running on this machine, " +
			"please install it (https://docs.docker.com/engine/installation/) " +
			"and make sure it can be used by a non-root user " +
			"(https://docs.docker.com/engine/installation/linux/linux-postinstall/)")
	}
	return nil
}
