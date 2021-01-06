// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

func push(path string, authDir string, force bool, insecure, quiet bool) error {
	// unpack the installer tarball into a temporary directory
	dir, err := ioutil.TempDir("", "tele")
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			log.Warnf("Failed to remove %v: %v.", dir, trace.DebugReport(err))
		}
	}()

	progress := utils.NewProgress(context.TODO(), "Push", 3, quiet)
	defer progress.Stop()

	reader, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	progress.NextStep("Unpacking installer tarball")

	err = archive.Extract(reader, dir)
	if err != nil {
		return trace.Wrap(err)
	}

	// create local environment using unpacked installer as a state dir
	env, err := localenv.NewLocalEnvironment(
		localenv.LocalEnvironmentArgs{
			StateDir:         dir,
			LocalKeyStoreDir: authDir,
			Insecure:         insecure,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	defer env.Close()

	opsURL, err := env.SelectOpsCenter("")
	if err != nil {
		return trace.Wrap(err)
	}

	apps, err := env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	app, err := install.GetAppPackage(apps)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.BadParameter("no valid application found in the tarball")
		}
		return trace.Wrap(err)
	}

	opsApps, err := env.AppService(opsURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	opsPackages, err := env.PackageService(opsURL)
	if err != nil {
		return trace.Wrap(err)
	}

	// make sure that the application we're about to upload does not exist in the target
	// OpsCenter unless "force" flag is provided
	if !force {
		existing, err := opsApps.GetApp(*app)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if existing != nil && !pack.IsMetadataPackage(existing.PackageEnvelope) {
			return trace.BadParameter(
				"application %v already exists in %v, please remove it first or provide '-f' flag to overwrite it", app, opsURL)
		}
	}

	targetOpsCenter, _ := utils.URLHostname(opsURL)
	progress.NextStep(fmt.Sprintf("Uploading %v:%v to %v", app.Name, app.Version, targetOpsCenter))

	puller := libapp.Puller{
		SrcPack: env.Packages,
		SrcApp:  apps,
		DstPack: opsPackages,
		DstApp:  opsApps,
		Upsert:  force,
	}
	err = puller.PullApp(context.TODO(), *app)
	if err != nil {
		return trace.Wrap(err)
	}

	progress.NextStep(fmt.Sprintf("Application %v:%v uploaded to %v", app.Name, app.Version, targetOpsCenter))

	return nil
}
