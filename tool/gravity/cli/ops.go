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
	"context"
	"fmt"
	"runtime"

	//nolint:gosec // imported for side-effects
	_ "net/http/pprof"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	libstatus "github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/gravitational/trace"
)

func appPackage(env *localenv.LocalEnvironment) error {
	apps, err := env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	appPackage, err := install.GetAppPackage(apps)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%v", appPackage)
	return nil
}

func uploadUpdate(ctx context.Context, tarballEnv *localenv.TarballEnvironment, env *localenv.LocalEnvironment) error {
	clusterOperator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err, "unable to access cluster.\n"+
			"Use 'gravity status' to check the cluster state and make sure "+
			"that the cluster DNS is working properly.")
	}

	cluster, err := clusterOperator.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	if cluster.State == ops.SiteStateDegraded {
		return trace.BadParameter("The cluster is in degraded state so " +
			"uploading new applications is prohibited. Please check " +
			"gravity status output and correct the situation before " +
			"attempting again.")
	}

	clusterPackages, err := env.ClusterPackages()
	if err != nil {
		return trace.Wrap(err)
	}

	clusterApps, err := env.AppServiceCluster()
	if err != nil {
		return trace.Wrap(err)
	}

	appPackage, err := install.GetAppPackage(tarballEnv.Apps)
	if err != nil {
		return trace.Wrap(err)
	}

	app, err := tarballEnv.Apps.GetApp(*appPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}

	var registries []string
	err = utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() (err error) {
		registries, err = getRegistries(ctx, cluster.ClusterState.Servers)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	imageServices := make([]docker.ImageService, 0, len(registries))
	for _, registryAddr := range registries {
		imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
			RegistryAddress: registryAddr,
			CertName:        defaults.DockerRegistry,
			CACertPath:      state.Secret(stateDir, defaults.RootCertFilename),
			ClientCertPath:  state.Secret(stateDir, "kubelet.cert"),
			ClientKeyPath:   state.Secret(stateDir, "kubelet.key"),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		imageServices = append(imageServices, imageService)
	}

	env.PrintStep("Importing application %v v%v", appPackage.Name, appPackage.Version)

	puller := libapp.Puller{
		FieldLogger: log.WithField(trace.Component, "pull"),
		SrcPack:     tarballEnv.Packages,
		SrcApp:      tarballEnv.Apps,
		DstPack:     clusterPackages,
		DstApp:      clusterApps,
		Upsert:      true,
		Parallel:    runtime.NumCPU(),
	}
	syncer := libapp.Syncer{
		PackService: tarballEnv.Packages,
		AppService:  tarballEnv.Apps,
		Progress:    env,
	}
	if err := uploadApplicationUpdate(ctx, puller, syncer, imageServices, *app); err != nil {
		return trace.Wrap(err)
	}

	// Uploading new blobs to the cluster is known to cause stress on disk
	// which can lead to the cluster's health checker experiencing momentary
	// blips and potentially moving the cluster to degraded state, especially
	// when running on a hardware with sub-par I/O performance.
	//
	// To accommodate this behavior and make sure upgrade (which normally
	// follows upload right away) does not fail to launch due to the degraded
	// state, give the cluster a few minutes to settle.
	//
	// See https://github.com/gravitational/gravity/issues/1659 for more info.
	env.PrintStep("Verifying cluster health")
	ctx, cancel := context.WithTimeout(ctx, defaults.NodeStatusTimeout)
	defer cancel()
	err = libstatus.WaitCluster(ctx, clusterOperator)
	if err != nil {
		return trace.Wrap(err)
	}

	env.PrintStep("Application has been uploaded")
	return nil
}

func uploadApplicationUpdate(ctx context.Context, puller libapp.Puller, syncer libapp.Syncer, imageServices []docker.ImageService, app libapp.Application) error {
	deps, err := getUploadDependencies(puller.SrcPack, puller.SrcApp, app)
	if err != nil {
		return trace.Wrap(err)
	}
	err = puller.Pull(ctx, *deps)
	if err != nil {
		return trace.Wrap(err)
	}
	err = puller.PullAppPackage(ctx, app.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	deps.Apps = append(deps.Apps, app)

	err = syncDependenciesWithCluster(ctx, imageServices, *deps, syncer)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getUploadDependencies(packages pack.PackageService, apps libapp.Applications, app libapp.Application) (*libapp.Dependencies, error) {
	deps, err := libapp.GetDependencies(libapp.GetDependenciesRequest{
		Pack: packages,
		Apps: apps,
		App:  app,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return deps, nil
}

func syncDependenciesWithCluster(ctx context.Context, imageServices []docker.ImageService, deps libapp.Dependencies, syncer libapp.Syncer) error {
	for _, imageService := range imageServices {
		syncer.Progress.PrintStep("Synchronizing application with Docker registry %v",
			imageService.String())

		syncer.ImageService = imageService
		err := syncer.Sync(ctx, deps)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func getTarballEnvironForUpgrade(env *localenv.LocalEnvironment, stateDir string) (*localenv.TarballEnvironment, error) {
	clusterOperator, err := env.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err, "unable to access cluster.\n"+
			"Use 'gravity status' to check the cluster state and make sure "+
			"that the cluster DNS is working properly.")
	}
	cluster, err := clusterOperator.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if stateDir == "" {
		// Use current working directory as state directory if unspecified
		stateDir = utils.Exe.WorkingDir
	}
	var license string
	if cluster.License != nil {
		license = cluster.License.Raw
	}
	return localenv.NewTarballEnvironment(localenv.TarballEnvironmentArgs{
		StateDir: stateDir,
		License:  license,
	})
}

// getRegistries returns a list of registry addresses in the cluster
func getRegistries(ctx context.Context, servers []storage.Server) ([]string, error) {
	// return registry addresses on all masters
	ips, err := getMasterNodes(ctx, servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	registries := make([]string, 0, len(ips))
	for _, ip := range ips {
		registries = append(registries, defaults.DockerRegistryAddr(ip))
	}
	return registries, nil
}

func connectToOpsCenter(env *localenv.LocalEnvironment, opsCenterURL, username, password string) (err error) {
	if username == "" || password == "" {
		username, password, err = common.ReadUserPass()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	entry, err := env.Backend.UpsertLoginEntry(
		storage.LoginEntry{
			OpsCenterURL: opsCenterURL,
			Email:        username,
			Password:     password})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("\n\nconnected to %v\n", *entry)
	return nil
}

// disconnectFromOpsCenter
func disconnectFromOpsCenter(env *localenv.LocalEnvironment, opsCenterURL string) error {
	err := env.Backend.DeleteLoginEntry(opsCenterURL)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	fmt.Printf("disconnected from %v", opsCenterURL)
	return nil
}

func listOpsCenters(env *localenv.LocalEnvironment) error {
	entries, err := env.Backend.GetLoginEntries()
	if err != nil {
		return trace.Wrap(err)
	}
	common.PrintHeader("logins")
	for _, entry := range entries {
		fmt.Printf("* %v %v\n", entry.OpsCenterURL, entry.Email)
	}
	fmt.Printf("\n")
	return nil
}
