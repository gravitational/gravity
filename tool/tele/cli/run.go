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
		return buildClusterImage(context.Background(), BuildParameters{
			StateDir:         *tele.StateDir,
			SourcePath:       *tele.BuildCmd.Path,
			OutPath:          *tele.BuildCmd.OutFile,
			Overwrite:        *tele.BuildCmd.Overwrite,
			SkipVersionCheck: *tele.BuildCmd.SkipVersionCheck,
			Silent:           *tele.BuildCmd.Quiet,
			Verbose:          *tele.BuildCmd.Verbose,
			BaseImage:        *tele.BuildCmd.BaseImage,
			Insecure:         *tele.Insecure,
			Vendor: service.VendorRequest{
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
			},
		})
	case tele.HelmBuildCmd.FullCommand():
		return buildApplicationImage(context.Background(), BuildParameters{
			StateDir:   *tele.StateDir,
			SourcePath: *tele.HelmBuildCmd.Path,
			OutPath:    *tele.HelmBuildCmd.OutFile,
			Overwrite:  *tele.HelmBuildCmd.Overwrite,
			Silent:     *tele.HelmBuildCmd.Quiet,
			Verbose:    *tele.HelmBuildCmd.Verbose,
			Insecure:   *tele.Insecure,
			Vendor: service.VendorRequest{
				ResourcePatterns:       *tele.HelmBuildCmd.VendorPatterns,
				IgnoreResourcePatterns: *tele.HelmBuildCmd.VendorIgnorePatterns,
				SetImages:              *tele.HelmBuildCmd.SetImages,
				Parallel:               *tele.HelmBuildCmd.Parallel,
				Helm: helm.RenderParameters{
					Values: *tele.HelmBuildCmd.Values,
					Set:    *tele.HelmBuildCmd.Set,
				},
				Pull: *tele.HelmBuildCmd.Pull,
			},
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
		return pull(
			*tele.PullCmd.App,
			*tele.PullCmd.OutFile,
			*tele.PullCmd.Force,
			*tele.PullCmd.Quiet)
	case tele.ListCmd.FullCommand():
		return list(
			*tele.ListCmd.All,
			*tele.ListCmd.Format)
	}

	return trace.NotFound("unknown command %v", cmd)
}
