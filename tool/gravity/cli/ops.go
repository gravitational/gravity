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
	_ "net/http/pprof"
	"time"

	appservice "github.com/gravitational/gravity/lib/app/service"
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

func uploadUpdate(ctx context.Context, tarballEnv *localenv.TarballEnvironment, env *localenv.LocalEnvironment, opsURL string, skipVersionCheck bool) error {
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
			"uploading new cluster images is prohibited. Please check " +
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

	application, err := install.GetApp(tarballEnv.Apps)
	if err != nil {
		return trace.Wrap(err)
	}

	var registries []string
	err = utils.RetryWithInterval(ctx, utils.NewExponentialBackOff(5*time.Minute), func() error {
		registries, err = getRegistries(ctx, env, cluster.ClusterState.Servers)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(registries) == 0 {
		return trace.NotFound("could not determine cluster registry addresses")
	}
	images, err := docker.NewClusterImageService(registries[0])
	if err != nil {
		return trace.Wrap(err)
	}

	sourcePackages := tarballEnv.Packages
	sourceApps := tarballEnv.Apps

	// Before importing, check if the new version is an incremental upgrade
	// and if so, check whether the currently installed version can be
	// upgraded to it.
	upgradeFrom, err := application.LabelAsLocator(pack.UpgradeFromLabel)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if upgradeFrom != nil {
		if !cluster.App.Package.IsEqualTo(*upgradeFrom) && !skipVersionCheck {
			return trace.BadParameter(`This cluster image was built as an incremental upgrade from %v which differs
from the currently installed cluster image %v.

Upgrading over a different image may result in unexpected behavior, such as a
failed upgrade due to missing Docker images.

If you wish to upgrade anyway, you can suppress this error by providing the
--skip-version-check flag.`,
				upgradeFrom.Description(),
				cluster.App.Package.Description())
		}
		// Incremental images currently require that the upgrade image has the
		// same base as the currently installed image. In future we may support
		// including partial runtime updates as well.
		if !application.Manifest.Base().IsEqualTo(*cluster.App.Manifest.Base()) {
			return trace.BadParameter(`This cluster image can only be used as an incremental upgrade for clusters
based on Gravity %v.

The currently installed cluster image %v is based on Gravity %v.`,
				application.Manifest.Base().Version,
				cluster.App.Manifest.Locator().Description(),
				cluster.App.Manifest.Base().Version)
		}
		response, err := appservice.RestoreApp(ctx, appservice.RestoreRequest{
			Packages:        sourcePackages,
			Apps:            sourceApps,
			ClusterPackages: clusterPackages,
			ClusterApps:     clusterApps,
			Images:          images,
			Locator:         application.Manifest.Locator(),
			Progress:        env,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer response.Cleanup()
		// Use package/app services where the restored app has been imported
		// as a source for importing into the cluster.
		sourcePackages = response.Packages
		sourceApps = response.Apps
	}

	env.PrintStep("Importing cluster image %v", application.Package.Description())
	_, err = appservice.PullApp(appservice.AppPullRequest{
		SrcPack: sourcePackages,
		SrcApp:  sourceApps,
		DstPack: clusterPackages,
		DstApp:  clusterApps,
		Package: application.Package,
	})
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		env.PrintStep("Cluster image already exists in local cluster")
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, registry := range registries {
		env.PrintStep("Synchronizing application with Docker registry %v",
			registry)

		imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
			RegistryAddress: registry,
			CertName:        defaults.DockerRegistry,
			CACertPath:      state.Secret(stateDir, defaults.RootCertFilename),
			ClientCertPath:  state.Secret(stateDir, "kubelet.cert"),
			ClientKeyPath:   state.Secret(stateDir, "kubelet.key"),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		err = appservice.SyncApp(ctx, appservice.SyncRequest{
			PackService:  sourcePackages,
			AppService:   sourceApps,
			ImageService: imageService,
			Package:      application.Package,
		})
		if err != nil {
			return trace.Wrap(err)
		}
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

	env.PrintStep("Cluster image has been uploaded")
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
func getRegistries(ctx context.Context, env *localenv.LocalEnvironment, servers []storage.Server) ([]string, error) {
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
