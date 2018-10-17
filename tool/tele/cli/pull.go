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

	"github.com/gravitational/gravity/lib/hub"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

func pull(env localenv.LocalEnvironment, app, outFile string, force, quiet bool) error {
	locator, err := pack.MakeLocator(app)
	if err != nil {
		return trace.Wrap(err)
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
		outFile = fmt.Sprintf("%v-%v.tar", locator.Name, locator.Version)
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

	progress := utils.NewProgress(context.TODO(), "Download", 1, quiet)
	defer progress.Stop()

	err = hub.Download(f, *locator, progress)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
