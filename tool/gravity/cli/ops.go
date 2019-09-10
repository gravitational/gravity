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
	"encoding/json"
	"fmt"
	_ "net/http/pprof"
	"runtime"
	"strings"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/docker"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

func selectNetworkInterface() (string, error) {
	for {
		addr, autoselected, err := selectInterface()
		if err != nil {
			return "", trace.Wrap(err)
		}
		if autoselected {
			return addr, nil
		}
		confirmed, err := confirmWithTitle(fmt.Sprintf(
			"\nConfirm the selected interface [%v]", addr))
		if err != nil {
			return "", trace.Wrap(err)
		}
		if !confirmed {
			continue
		}
		return addr, nil
	}
}

func mustJSON(i interface{}) string {
	bytes, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

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

func uploadUpdate(_ *localenv.LocalEnvironment, opsURL, dataDir string) error {
	// Create local environment with explicit gravity state directory since the env
	// might be pointing to the current working directory in case of the old upload
	// script
	localStateDir, err := localenv.LocalGravityDir()
	if err != nil {
		return trace.Wrap(err)
	}

	defaultEnv, err := localenv.New(localStateDir)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterOperator, err := defaultEnv.SiteOperator()
	if err != nil {
		return trace.Wrap(err, "unable to access cluster.\n"+
			"Use 'gravity status' to check the cluster state and make sure "+
			"that the cluster DNS is working properly.")
	}

	cluster, err := clusterOperator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	args := localenv.TarballEnvironmentArgs{StateDir: dataDir}
	if cluster.License != nil {
		args.License = cluster.License.Raw
	}
	tarballEnv, err := localenv.NewTarballEnvironment(args)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tarballEnv.Close()

	appPackage, err := install.GetAppPackage(tarballEnv.Apps)
	if err != nil {
		return trace.Wrap(err)
	}

	err = canUpload(*cluster, *appPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterPackages, err := defaultEnv.ClusterPackages()
	if err != nil {
		return trace.Wrap(err)
	}

	clusterApps, err := defaultEnv.SiteApps()
	if err != nil {
		return trace.Wrap(err)
	}

	installedRuntime := cluster.App.Manifest.Base()
	if installedRuntime == nil {
		return trace.BadParameter("failed to determine version of base image")
	}
	installedRuntimeVersion, err := installedRuntime.SemVer()
	if err != nil {
		return trace.Wrap(err)
	}

	deps, err := getUploadDependencies(tarballEnv, *appPackage, *installedRuntimeVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	defaultEnv.PrintStep("Importing cluster image %v v%v", appPackage.Name, appPackage.Version)
	puller := libapp.Puller{
		FieldLogger: log.WithField(trace.Component, "pull"),
		SrcPack:     tarballEnv.Packages,
		SrcApp:      tarballEnv.Apps,
		DstPack:     clusterPackages,
		DstApp:      clusterApps,
		Upsert:      true,
		Parallel:    runtime.NumCPU(),
	}
	err = puller.Pull(context.TODO(), *deps)
	if err != nil {
		return trace.Wrap(err)
	}

	syncer := libapp.Syncer{
		PackService: clusterPackages,
		AppService:  clusterApps,
	}
	appDeps, err := libapp.GetAppDependencies(*appPackage, clusterApps, clusterPackages)
	if err != nil {
		return trace.Wrap(err)
	}
	deps.Apps = append(deps.Apps, appDeps.Apps...)
	err = syncDependenciesWithCluster(context.TODO(), defaultEnv, *cluster, *deps, syncer)
	if err != nil {
		return trace.Wrap(err)
	}

	defaultEnv.PrintStep("Cluster image has been uploaded")
	return nil
}

func getUploadDependencies(env *localenv.TarballEnvironment, loc loc.Locator, installedRuntimeVersion semver.Version) (*libapp.Dependencies, error) {
	app, err := env.Apps.GetApp(loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	deps, err := libapp.GetDependencies(libapp.GetDependenciesRequest{
		Pack: env.Packages,
		Apps: env.Apps,
		App:  *app,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	deps.Apps = append(deps.Apps, *app)
	err = collectUpgradeDependencies(env, installedRuntimeVersion, deps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return deps, nil
}

func collectUpgradeDependencies(env *localenv.TarballEnvironment, installedRuntimeVersion semver.Version, deps *libapp.Dependencies) error {
	return pack.ForeachPackage(env.Packages, func(pkg pack.PackageEnvelope) error {
		version, ok := pkg.RuntimeLabels[pack.PurposeRuntimeUpgrade]
		if !ok {
			return nil
		}
		runtimeVersion, err := semver.NewVersion(version)
		if err != nil {
			return trace.Wrap(err, "invalid semver %q for upgrade package %v",
				version, pkg)
		}
		if installedRuntimeVersion.Compare(*runtimeVersion) > 0 {
			// Do not consider packages for runtime version lower than
			// or equal to the installed one
			return nil
		}
		if pkg.Type == "" {
			deps.Packages = append(deps.Packages, pkg)
			return nil
		}
		app, err := env.Apps.GetApp(pkg.Locator)
		if err != nil {
			return trace.Wrap(err)
		}
		deps.Apps = append(deps.Apps, *app)
		return nil
	})
}

func syncDependenciesWithCluster(ctx context.Context, env *localenv.LocalEnvironment, cluster ops.Site, deps libapp.Dependencies, syncer libapp.Syncer) error {
	var registries []string
	err := utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() (err error) {
		registries, err = getRegistries(ctx, env, cluster.ClusterState.Servers)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
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
			CertName:        constants.DockerRegistry,
			CACertPath:      state.Secret(stateDir, defaults.RootCertFilename),
			ClientCertPath:  state.Secret(stateDir, "kubelet.cert"),
			ClientKeyPath:   state.Secret(stateDir, "kubelet.key"),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		syncer.ImageService = imageService
		err = syncer.Sync(ctx, deps)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func canUpload(cluster ops.Site, app loc.Locator) error {
	if cluster.State == ops.SiteStateDegraded {
		return trace.BadParameter("The cluster is in degraded state so " +
			"uploading a new cluster image is prohibited. Please check " +
			"gravity status output and correct the situation before " +
			"attempting again.")
	}
	return pack.CheckUpdatePackage(cluster.App.Package, app)
}

// getRegistries returns a list of registry addresses in the cluster
func getRegistries(ctx context.Context, env *localenv.LocalEnvironment, servers []storage.Server) ([]string, error) {
	// in planets before certain version registry was running only on active master
	version, err := planetVersion(env)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if version.LessThan(*constants.PlanetMultiRegistryVersion) {
		return []string{constants.DockerRegistry}, nil
	}
	// otherwise return registry addresses on all masters
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

// connectToOpsCenter
func connectToOpsCenter(env *localenv.LocalEnvironment, opsCenterURL, username, password string) (err error) {
	if username == "" || password == "" {
		username, password, err = common.ReadUserPass()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	entry, err := env.Creds.UpsertLoginEntry(
		users.LoginEntry{
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
	err := env.Creds.DeleteLoginEntry(opsCenterURL)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	fmt.Printf("disconnected from %v", opsCenterURL)
	return nil
}

func listOpsCenters(env *localenv.LocalEnvironment) error {
	entries, err := env.Creds.GetLoginEntries()
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

type envvars map[string]string

func newEnvironSource(env []string) (result envvars) {
	result = make(map[string]string)
	for _, variable := range env {
		keyvalue := strings.Split(variable, "=")
		if len(keyvalue) == 2 {
			key, value := keyvalue[0], keyvalue[1]
			result[key] = value
		}
	}
	return result
}

func (r envvars) GetEnv(name string) string {
	return r[name]
}
