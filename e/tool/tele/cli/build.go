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
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/tele/cli"

	"github.com/gravitational/trace"
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
}

// build builds an installer tarball according to the provided parameters
func build(ctx context.Context, params buildParameters, req service.VendorRequest) (err error) {
	generator, err := builder.NewGenerator(builder.Config{
		RemoteSupportAddress: params.RemoteSupportAddress,
		RemoteSupportToken:   params.RemoteSupportToken,
		CACertPath:           params.CACertPath,
		EncryptionKey:        params.EncryptionKey,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	builder, err := basebuilder.New(basebuilder.Config{
		Context:          ctx,
		StateDir:         params.StateDir,
		Insecure:         params.Insecure,
		ManifestPath:     params.ManifestPath,
		OutPath:          params.OutPath,
		Overwrite:        params.Overwrite,
		Repository:       params.Repository,
		SkipVersionCheck: params.SkipVersionCheck,
		VendorReq:        req,
		NewSyncer:        builder.NewSyncer,
		Generator:        generator,
		GetRepository:    builder.GetRepository,
		Progress:         utils.NewProgress(ctx, "Build", 6, params.Silent),
		Silent:           params.Silent,
		UpgradeVia:       params.UpgradeVia,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer builder.Close()
	return basebuilder.Build(ctx, builder)
}
