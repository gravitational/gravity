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
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
)

// outputAppHook runs an application hook and prints the output
func outputAppHook(env *localenv.LocalEnvironment, req appservice.HookRunRequest) error {
	out, err := runAppHook(env, req)
	fmt.Printf("%s", out)
	return trace.Wrap(err)
}

// runAppHook runs an application hook specified with hook
func runAppHook(env *localenv.LocalEnvironment, req appservice.HookRunRequest) (string, error) {
	registryURL, err := localAppEnviron()
	if err != nil {
		return "", trace.Wrap(err)
	}
	apps, err := env.AppServiceLocal(localenv.AppConfig{RegistryURL: registryURL})
	if err != nil {
		return "", trace.Wrap(err)
	}
	_, out, err := appservice.RunAppHook(context.TODO(), apps, req)
	if err != nil {
		return string(out), trace.Wrap(err, "failed to run hook: %s", out)
	}
	return string(out), nil
}

// statusApp prints application status in json format
func statusApp(env *localenv.LocalEnvironment, appPackage loc.Locator, portalURL string) error {
	registryURL, err := localAppEnviron()
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := env.AppServiceLocal(localenv.AppConfig{RegistryURL: registryURL})
	if err != nil {
		return trace.Wrap(err)
	}
	status, err := apps.StatusApp(appPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	statusBytes, err := json.Marshal(status)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s", statusBytes)
	return nil
}

// listApps lists installed applications
func listApps(env *localenv.LocalEnvironment, repository, appType string, showHidden bool, opsCenterURL string) error {
	apps, err := env.AppService(opsCenterURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	packageService, err := env.PackageService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = foreachRepository(repository, packageService, func(repository string) error {
		installedApps, err := apps.ListApps(appservice.ListAppsRequest{
			Repository:    repository,
			Type:          storage.AppType(appType),
			ExcludeHidden: !showHidden,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		common.PrintHeader(repository)
		for _, app := range installedApps {
			fmt.Printf("* %v\n", app.String())
		}
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// uninstallAppPackage uninstalls gravity application from cluster
func uninstallAppPackage(env *localenv.LocalEnvironment, appPackage loc.Locator) error {
	apps, err := env.SiteApps()
	if err != nil {
		return trace.Wrap(err)
	}

	app, err := apps.UninstallApp(appPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("%v uninstalled\n", app)
	return nil
}

// importApp imports an application from the specified directory creating a new
// package named packageName.
func importApp(env *localenv.LocalEnvironment, registryURL, dockerURL, source string, req *appservice.ImportRequest,
	opsCenterURL string, silent bool, parallel int) error {
	apps, err := env.AppService(opsCenterURL, localenv.AppConfig{
		DockerURL:   dockerURL,
		RegistryURL: registryURL,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	packages, err := env.PackageService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	fileInfo, err := os.Stat(source)
	if err != nil {
		return trace.Wrap(err)
	}
	steps := 3
	if req.Vendor {
		steps += 1
	}
	progress := utils.NewProgress(context.TODO(), "app import", steps, silent)
	defer progress.Stop()

	var stream io.ReadCloser
	if fileInfo.IsDir() {
		progress.NextStep("importing directory %v", source)
		if stream, err = dockerarchive.Tar(source, dockerarchive.Uncompressed); err != nil {
			return trace.Wrap(err)
		}
	} else {
		progress.NextStep("importing archive %v", source)
		if stream, err = os.Open(source); err != nil {
			return trace.Wrap(err)
		}
	}

	if req.Vendor {
		progress.NextStep("vendoring docker images from %v", source)
		vendorer, err := service.NewVendorer(service.VendorerConfig{
			DockerURL:   dockerURL,
			RegistryURL: registryURL,
			Packages:    packages,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		if parallel == 0 {
			parallel = runtime.NumCPU()
		}
		unpackedDir, err := vendorer.VendorTarball(context.TODO(), stream, service.VendorRequest{
			Repository:             req.Repository,
			PackageName:            req.PackageName,
			PackageVersion:         req.PackageVersion,
			ResourcePatterns:       req.ResourcePatterns,
			IgnoreResourcePatterns: req.IgnoreResourcePatterns,
			SetImages:              req.SetImages,
			SetDeps:                req.SetDeps,
			Parallel:               parallel,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer os.RemoveAll(unpackedDir)
		// this stream now points to the tarball with the vendored app
		if stream, err = dockerarchive.Tar(unpackedDir, dockerarchive.Uncompressed); err != nil {
			return trace.Wrap(err)
		}
	}

	progressC := make(chan *appservice.ProgressEntry)
	errorC := make(chan error, 1)
	req.Source = stream
	req.ProgressC = progressC
	req.ErrorC = errorC
	op, err := apps.CreateImportOperation(req)
	if err != nil {
		return trace.Wrap(err)
	}

	progress.NextStep("importing application")

	for entry := range progressC {
		progress.UpdateCurrentStep("%v %v", entry.Message, entry.Completion)
	}

	if err = <-errorC; err != nil {
		return trace.Wrap(err, "failed to import %v", source)
	}

	app, err := apps.GetImportedApplication(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	progress.NextStep("%v imported", app.Package)
	return nil
}

// exportApp exports containers of the specified application package packageName
// to the private docker registry identified with registryHostPort
func exportApp(env *localenv.LocalEnvironment, packageName, portalURL, registryHostPort string) error {
	apps, err := env.AppService(portalURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	locator, err := loc.ParseLocator(packageName)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = apps.ExportApp(appservice.ExportAppRequest{
		Package:         *locator,
		RegistryAddress: registryHostPort,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%s exported to `%v`\n", packageName, registryHostPort)
	return nil
}

// deleteApp deletes an application record from database
func deleteApp(env *localenv.LocalEnvironment, packageName, portalURL string, force bool) error {
	locator, err := loc.ParseLocator(packageName)
	if err != nil {
		return trace.Wrap(err)
	}

	apps, err := env.AppService(portalURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	req := appservice.DeleteRequest{
		Package: *locator,
		Force:   force,
	}

	if err = apps.DeleteApp(req); err != nil {
		if force && trace.IsNotFound(err) {
			log.Warningf(err.Error())
			return nil
		}
		return trace.Wrap(err)
	}

	var source string
	if portalURL != "" {
		source = fmt.Sprintf("from %v", portalURL)
	}

	fmt.Printf("%v has been deleted %v\n", packageName, source)
	return nil
}

func pullApp(env *localenv.LocalEnvironment, appPackage loc.Locator, portalURL string, labels map[string]string, force bool) error {
	log.Infof("Pulling app package %v from %v.", appPackage, portalURL)

	remoteApps, err := env.AppService(portalURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	remoteApp, err := remoteApps.GetApp(appPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	localApps, err := env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	remotePackages, err := env.PackageService(portalURL)
	if err != nil {
		return trace.Wrap(err)
	}

	puller := appservice.Puller{
		SrcPack:  remotePackages,
		DstPack:  env.Packages,
		SrcApp:   remoteApps,
		DstApp:   localApps,
		Labels:   labels,
		Progress: env.Reporter,
		Upsert:   force,
	}
	err = puller.PullApp(context.TODO(), appPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("%v pulled from %v\n", remoteApp, portalURL)
	return nil
}

func pushApp(env *localenv.LocalEnvironment, appPackage loc.Locator, portalURL string) error {
	log.Infof("pushing app package %v to %v", appPackage, portalURL)

	localApps, err := env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	localApp, err := localApps.GetApp(appPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	remoteApps, err := env.AppService(portalURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	remotePackages, err := env.PackageService(portalURL)
	if err != nil {
		return trace.Wrap(err)
	}

	puller := appservice.Puller{
		SrcPack:  env.Packages,
		DstPack:  remotePackages,
		SrcApp:   localApps,
		DstApp:   remoteApps,
		Progress: env.Reporter,
	}
	err = puller.PullApp(context.TODO(), appPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("%v pushed to %v\n", localApp, portalURL)
	return nil
}

func unpackAppResources(env *localenv.LocalEnvironment, loc loc.Locator, dir, opsCenterURL, serviceUID string) error {
	apps, err := env.AppService(opsCenterURL, localenv.AppConfig{}, httplib.WithDialTimeout(dialTimeout))
	if err != nil {
		return trace.Wrap(err)
	}

	if dir == "" {
		unpackedDir := filepath.Join(env.StateDir, defaults.PackagesDir, defaults.UnpackedDir)
		dir = pack.PackagePath(unpackedDir, loc)
	}

	isDir, err := utils.IsDirectory(dir)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	} else {
		if !isDir {
			return trace.BadParameter("%v is not directory", dir)
		}
	}

	packageService, err := env.PackageService(opsCenterURL, httplib.WithDialTimeout(dialTimeout))
	if err != nil {
		return trace.Wrap(err)
	}

	locPtr, err := pack.ProcessMetadata(packageService, &loc)
	if err != nil {
		return trace.Wrap(err)
	}
	loc = *locPtr

	// Make sure package exists
	if _, err := packageService.ReadPackageEnvelope(loc); err != nil {
		return trace.Wrap(err)
	}

	if err := os.MkdirAll(dir, defaults.SharedDirMask); err != nil {
		return trace.Wrap(err)
	}

	err = utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
		reader, err := apps.GetAppResources(loc)
		if err != nil {
			return trace.Wrap(err)
		}
		defer reader.Close()

		return archive.ExtractWithPrefix(reader, dir, defaults.ResourcesDir)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	uid := defaults.ServiceUID
	if serviceUID != "" {
		uid, err = strconv.Atoi(serviceUID)
		if err != nil {
			return trace.BadParameter("invalid numeric user ID %q: %v", serviceUID, err)
		}
	}
	err = resources.UpdateSecurityContextInDir(dir, systeminfo.User{UID: uid})
	if err != nil {
		return trace.Wrap(err, "failed to render application resources")
	}

	env.Printf("%v unpacked at %v\n", loc, dir)
	return nil
}

func localAppEnviron() (registryHostPort string, err error) {
	host, err := pickSiteHost()
	if err != nil {
		return "", trace.Wrap(err)
	}
	registryHostPort = fmt.Sprintf("%v:5000", host)
	return registryHostPort, nil
}

func ensureNoApp(locator loc.Locator, apps appservice.Applications) error {
	app, err := apps.GetApp(locator)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		return nil
	}
	return trace.AlreadyExists("%v already exists", app)
}
