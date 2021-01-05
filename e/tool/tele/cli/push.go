package cli

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/tele/cli"

	"github.com/gravitational/trace"
)

func push(env *localenv.LocalEnvironment, path string, force, quiet bool) error {
	progress := cli.NewProgress(context.TODO(), "Push", quiet)
	defer progress.Stop()

	clusterURL, err := env.SelectOpsCenter("")
	if err != nil {
		return trace.Wrap(err)
	}

	progress.NextStep("Using Hub %v", clusterURL)

	reader, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	// Create a temporary directory where cluster image will be unpacked.
	dir, err := ioutil.TempDir("", "tele")
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			log.WithError(err).Warnf("Failed to remove %v.", dir)
		}
	}()

	// Unpack the cluster image into the temporary directory.
	progress.NextStep("Unpacking image into %v", dir)
	err = archive.Extract(reader, dir)
	if err != nil {
		return trace.Wrap(err)
	}

	// Create the local environment pointing to the unpacked image.
	unpackedEnv, err := localenv.NewTarballEnvironment(localenv.TarballEnvironmentArgs{
		StateDir: dir,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Determine the application we're about to push.
	app, err := install.GetAppPackage(unpackedEnv.Apps)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.BadParameter("unpacked tarball does not contain a " +
				"cluster or application image")
		}
		return trace.Wrap(err)
	}

	// Get clients for the remote cluster we're currently logged into.
	clusterPackages, err := env.PackageService(clusterURL)
	if err != nil {
		return trace.Wrap(err)
	}
	clusterApps, err := env.AppService(clusterURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	// Make sure the application we're about to upload does not exist in the
	// cluster, unless "force" flag is provided.
	if !force {
		existing, err := clusterApps.GetApp(*app)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if existing != nil && !service.IsMetadataPackage(existing.PackageEnvelope) {
			return trace.AlreadyExists("image %v already exists in %v, please "+
				"remove it first or provide --force flag to overwrite it",
				app, clusterURL)
		}
	}

	// Finally, push.
	targetCluster, _ := utils.URLHostname(clusterURL)
	progress.NextStep(fmt.Sprintf("Uploading %v:%v to %v",
		app.Name, app.Version, targetCluster))
	_, err = service.PullApp(service.AppPullRequest{
		SrcPack: unpackedEnv.Packages,
		SrcApp:  unpackedEnv.Apps,
		DstPack: clusterPackages,
		DstApp:  clusterApps,
		Package: *app,
		Upsert:  force,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	progress.NextStep(fmt.Sprintf("Image %v:%v uploaded to %v",
		app.Name, app.Version, targetCluster))
	return nil
}
