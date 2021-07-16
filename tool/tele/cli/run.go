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
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/localenv"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "cli")

// Run parses CLI arguments and executes an appropriate tele command
func Run(tele Application) error {
	log.Debugf("Executing: %v.", os.Args)
	cmd, err := tele.Parse(os.Args[1:])
	if err != nil {
		return trace.Wrap(err)
	}

	trace.SetDebug(*tele.Debug)
	if *tele.Debug {
		teleutils.InitLogger(teleutils.LoggingForDaemon, logrus.DebugLevel)
	} else {
		teleutils.InitLogger(teleutils.LoggingForCLI, logrus.InfoLevel)
	}

	switch cmd {
	case tele.VersionCmd.FullCommand():
		return printVersion(*tele.VersionCmd.Output)
	case tele.BuildCmd.FullCommand():
		return build(context.Background(), BuildParameters{
			StateDir:         *tele.StateDir,
			ManifestPath:     *tele.BuildCmd.ManifestPath,
			OutPath:          *tele.BuildCmd.OutFile,
			Overwrite:        *tele.BuildCmd.Overwrite,
			SkipVersionCheck: *tele.BuildCmd.SkipVersionCheck,
			Silent:           *tele.BuildCmd.Quiet,
			Verbose:          *tele.BuildCmd.Verbose,
			Insecure:         *tele.Insecure,
			UpgradeVia:       *tele.BuildCmd.UpgradeVia,
		}, service.VendorRequest{
			PackageName:            *tele.BuildCmd.Name,
			PackageVersion:         *tele.BuildCmd.Version,
			ResourcePatterns:       *tele.BuildCmd.VendorPatterns,
			IgnoreResourcePatterns: *tele.BuildCmd.VendorIgnorePatterns,
			ImageCacheDir:          *tele.BuildCmd.ImageCacheDir,
			SetImages:              *tele.BuildCmd.SetImages,
			SetDeps:                *tele.BuildCmd.SetDeps,
			Parallel:               *tele.BuildCmd.Parallel,
			VendorRuntime:          true,
			Helm: helm.RenderParameters{
				Values: *tele.BuildCmd.Values,
				Set:    *tele.BuildCmd.Set,
			},
			Pull: *tele.BuildCmd.Pull,
		})
	}

	keystoreDir := *tele.StateDir
	if *tele.StateDir == "" {
		*tele.StateDir, err = ioutil.TempDir("", "tele")
		if err != nil {
			return trace.Wrap(err)
		}
		defer os.RemoveAll(*tele.StateDir)
	}

	env, err := localenv.NewLocalEnvironment(
		localenv.LocalEnvironmentArgs{
			StateDir:         *tele.StateDir,
			LocalKeyStoreDir: keystoreDir,
			Insecure:         *tele.Insecure,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	defer env.Close()

	switch cmd {
	case tele.PullCmd.FullCommand():
		return pull(*env,
			*tele.PullCmd.App,
			*tele.PullCmd.OutFile,
			*tele.PullCmd.Force,
			*tele.PullCmd.Quiet)
	case tele.ListCmd.FullCommand():
		return list(*env,
			*tele.ListCmd.All,
			*tele.ListCmd.Format)
	}

	return trace.NotFound("unknown command %v", cmd)
}
