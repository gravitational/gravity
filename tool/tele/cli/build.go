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

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/builder"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// BuildParameters represents the arguments provided for building an application
type BuildParameters struct {
	// BuildEnv is the local environment used to build the application
	BuildEnv *localenv.LocalEnvironment
	// StateDir is build state directory, if was specified
	StateDir string
	// ManifestPath holds the path to the application manifest
	ManifestPath string
	// OutPath holds the path to the installer tarball to be output
	OutPath string
	// Overwrite indicates whether or not to overwrite an existing installer file
	Overwrite bool
	// Repository represents the source package repository
	Repository string
	// SkipVersionCheck indicates whether or not to perform the version check of the tele binary with the application's runtime at build time
	SkipVersionCheck bool
	// Silent is whether builder should report progress to the console
	Silent bool
}

// build builds an installer tarball according to the provided parameters
func build(ctx context.Context, params BuildParameters, req service.VendorRequest) (err error) {
	var syncer builder.Syncer
	if params.StateDir != "" {
		// if state directory was explicitly provided, use it as package source
		syncer, err = builder.NewLocalPackSyncer(params.BuildEnv)
	} else {
		// otherwise sync packages with S3 bucket
		syncer, err = builder.NewS3Syncer()
	}
	if err != nil {
		return trace.Wrap(err)
	}
	installerBuilder, err := builder.New(builder.Config{
		Context:          ctx,
		Env:              params.BuildEnv,
		ManifestPath:     params.ManifestPath,
		OutPath:          params.OutPath,
		Overwrite:        params.Overwrite,
		Repository:       params.Repository,
		SkipVersionCheck: params.SkipVersionCheck,
		VendorReq:        req,
		Syncer:           syncer,
		Progress:         utils.NewProgress(ctx, "Build", 6, params.Silent),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer installerBuilder.Close()
	return builder.Build(ctx, installerBuilder)
}
