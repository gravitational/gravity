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
	"io"
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

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

	locator, err := loc.MakeLocator(app)
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

	progress := utils.NewProgress(context.TODO(), "Download", 3, quiet)
	defer progress.Stop()

	progress.NextStep("Requesting image from %v", opsURL)

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

	progress.NextStep(fmt.Sprintf("Image %v downloaded", app))

	return nil
}
