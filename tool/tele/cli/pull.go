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
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/hub"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// NewProgress returns progress reporter used in tele cli.
func NewProgress(ctx context.Context, title string, silent bool) utils.Progress {
	level := utils.ProgressLevelInfo
	if silent {
		level = utils.ProgressLevelNone
	}
	return utils.NewProgressWithConfig(ctx, title, utils.ProgressConfig{
		Level:       level,
		StepPrinter: utils.TimestampedStepPrinter,
	})
}

// MakeLocator creates locator from the provided application package name.
func MakeLocator(app string) (*loc.Locator, error) {
	return loc.MakeLocatorWithDefault(app, func(name string) string {
		switch name {
		case constants.BaseImageName, constants.LegacyBaseImageName, constants.HubImageName, constants.LegacyHubImageName:
			// For system images (base and hub) default to tele version for compatibility.
			return modules.Get().Version().Version
		default:
			// For everything else (user images) default to the latest.
			return loc.LatestVersion
		}
	})
}

func pull(env localenv.LocalEnvironment, app, outFile string, force, quiet bool) error {
	locator, err := MakeLocator(app)
	if err != nil {
		return trace.Wrap(err)
	}
	name := locator.Name

	// tele ls displays base images as "gravity" while the actual image
	// name is "telekube" (for legacy reasons).
	if locator.Name == constants.BaseImageName {
		locator.Name = constants.LegacyBaseImageName
	}

	hub, err := hub.New(hub.Config{})
	if err != nil {
		return trace.Wrap(err)
	}

	if locator.Version == loc.LatestVersion {
		locator.Version, err = hub.GetLatestVersion(locator.Name)
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
		return trace.AlreadyExists("file %v already exists, provide '--force'"+
			"flag to overwrite it", outFile)
	}

	f, err := os.Create(outFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	progress := NewProgress(context.TODO(), "Download", quiet)
	defer progress.Stop()

	progress.NextStep(fmt.Sprintf("Downloading %v:%v", name, locator.Version))

	err = hub.Download(f, *locator)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
