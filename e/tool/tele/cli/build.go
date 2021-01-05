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

	"github.com/gravitational/gravity/e/lib/builder"
	"github.com/gravitational/gravity/lib/app/service"
	basebuilder "github.com/gravitational/gravity/lib/builder"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/localenv/credentials"
	"github.com/gravitational/gravity/tool/tele/cli"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// buildParameters extends CLI parameters for open-source version of tele build
type buildParameters struct {
	// BuildParameters is the build parameters from open-source
	cli.BuildParameters
	// RemoteSupport is an optional remote Ops Center address
	RemoteSupportAddress string
	// RemoteSupport is an optional remote Ops Center token
	RemoteSupportToken string
	// CACertPath is an optional path to CA certificate installer will use
	CACertPath string
	// EncryptionKey is an optional key used to encrypt installer packages at rest
	EncryptionKey string
	// Credentials is the optional credentials set on the CLI
	Credentials *credentials.Credentials
}

func (p buildParameters) repository(service credentials.Service) (repository string, err error) {
	if p.Credentials != nil {
		return p.Credentials.URL, nil
	}
	// look for an cluster we're logged into
	credentials, err := service.Current()
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}
	// otherwise use the default one
	if trace.IsNotFound(err) {
		return defaults.DistributionOpsCenter, nil
	}
	return credentials.URL, nil
}

// build builds an installer tarball according to the provided parameters
func build(ctx context.Context, params buildParameters, req service.VendorRequest) error {
	generator, err := builder.NewGenerator(builder.Config{
		RemoteSupportAddress: params.RemoteSupportAddress,
		RemoteSupportToken:   params.RemoteSupportToken,
		CACertPath:           params.CACertPath,
		EncryptionKey:        params.EncryptionKey,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	creds, err := credentials.New(credentials.Config{
		LocalKeyStoreDir: params.StateDir,
		Credentials:      params.Credentials,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	repository, err := params.repository(creds)
	if err != nil {
		return trace.Wrap(err)
	}
	logger := logrus.WithField(trace.Component, "builder")
	env, err := params.newBuildEnviron(repository, logger)
	if err != nil {
		return trace.Wrap(err)
	}
	pack, err := env.PackageService(repository)
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := env.AppService(repository, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	builder, err := basebuilder.New(basebuilder.Config{
		Context:          ctx,
		Env:              env,
		FieldLogger:      logger,
		Insecure:         params.Insecure,
		ManifestPath:     params.ManifestPath,
		OutPath:          params.OutPath,
		Overwrite:        params.Overwrite,
		Credentials:      params.Credentials,
		SkipVersionCheck: params.SkipVersionCheck,
		VendorReq:        req,
		Repository:       repository,
		Syncer:           basebuilder.NewPackSyncer(pack, apps),
		Generator:        generator,
		Progress:         params.Progress(ctx),
		UpgradeVia:       params.UpgradeVia,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer builder.Close()
	return builder.Build(ctx)
}

func (p buildParameters) newBuildEnviron(repository string, logger logrus.FieldLogger) (*localenv.LocalEnvironment, error) {
	// if state directory was specified explicitly, it overrides
	// both cache directory and config directory as it's used as
	// a special case only for building from local packages
	if p.StateDir != "" {
		logger.Infof("Using package cache from %v.", p.StateDir)
		return localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
			StateDir:         p.StateDir,
			LocalKeyStoreDir: p.StateDir,
			Insecure:         p.Insecure,
			Credentials:      p.Credentials,
		})
	}
	// otherwise use default locations for cache / key store
	cacheDir, err := basebuilder.EnsureCacheDir(repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger.Infof("Using package cache from %v.", cacheDir)
	return localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
		StateDir:    cacheDir,
		Insecure:    p.Insecure,
		Credentials: p.Credentials,
	})
}
