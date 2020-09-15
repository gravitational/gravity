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

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/builder"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
)

// BuildParameters represents the arguments provided for building an application
type BuildParameters struct {
	// StateDir is an optional build state directory
	StateDir string
	// ManifestPath holds the path to the application manifest
	ManifestPath string
	// OutPath holds the path to the installer tarball to be output
	OutPath string
	// Overwrite indicates whether or not to overwrite an existing installer file
	Overwrite bool
	// SkipVersionCheck indicates whether or not to perform the version check of the tele binary with the application's runtime at build time
	SkipVersionCheck bool
	// Silent is whether builder should report progress to the console
	Silent bool
	// Verbose turns on more detailed progress output
	Verbose bool
	// Insecure turns on insecure verify mode
	Insecure bool
	// UpgradeVia lists intermediate runtime versions to embed inside the installer
	UpgradeVia []string
}

// Progress creates the progress based on the CLI parameters.
func (p BuildParameters) Progress(ctx context.Context) utils.Progress {
	// Normal output.
	level := utils.ProgressLevelInfo
	if p.Silent { // No output.
		level = utils.ProgressLevelNone
	} else if p.Verbose { // Detailed output.
		level = utils.ProgressLevelDebug
	}
	return utils.NewProgressWithConfig(ctx, "Build", utils.ProgressConfig{
		Level:       level,
		StepPrinter: utils.TimestampedStepPrinter,
	})
}

// build builds an installer tarball according to the provided parameters
func build(ctx context.Context, params BuildParameters, req service.VendorRequest) error {
	syncer, err := builder.NewS3Syncer()
	if err != nil {
		return trace.Wrap(err)
	}
	logger := logrus.WithField(trace.Component, "builder")
	env, err := params.newBuildEnviron(logger)
	if err != nil {
		return trace.Wrap(err)
	}
	installerBuilder, err := builder.New(builder.Config{
		Context:          ctx,
		Env:              env,
		FieldLogger:      logger,
		Insecure:         params.Insecure,
		ManifestPath:     params.ManifestPath,
		OutPath:          params.OutPath,
		Overwrite:        params.Overwrite,
		SkipVersionCheck: params.SkipVersionCheck,
		VendorReq:        req,
		Progress:         params.Progress(ctx),
		UpgradeVia:       params.UpgradeVia,
		Repository:       getRepository(),
		Syncer:           syncer,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer installerBuilder.Close()
	return installerBuilder.Build(ctx)
}

func (p BuildParameters) newBuildEnviron(logger logrus.FieldLogger) (*localenv.LocalEnvironment, error) {
	// if state directory was specified explicitly, it overrides
	// both cache directory and config directory as it's used as
	// a special case only for building from local packages
	if p.StateDir != "" {
		logger.Infof("Using package cache from %v.", p.StateDir)
		return localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
			StateDir:         p.StateDir,
			LocalKeyStoreDir: p.StateDir,
			Insecure:         p.Insecure,
		})
	}
	// otherwise use default locations for cache / key store
	cacheDir, err := builder.EnsureCacheDir(getRepository())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger.Infof("Using package cache from %v.", cacheDir)
	return localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
		StateDir: cacheDir,
		Insecure: p.Insecure,
	})
}

// getRepository returns the default package source repository
func getRepository() string {
	return fmt.Sprintf("s3://%v", defaults.HubBucket)
}
