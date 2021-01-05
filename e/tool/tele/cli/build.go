package cli

import (
	"context"

	"github.com/gravitational/gravity/e/lib/builder"
	basebuilder "github.com/gravitational/gravity/lib/builder"
	"github.com/gravitational/gravity/lib/localenv/credentials"
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
	// Credentials is the optional credentials set on the CLI
	Credentials *credentials.Credentials
}

func (p buildParameters) repository() string {
	if p.Credentials != nil {
		return p.Credentials.URL
	}
	return ""
}

func (p buildParameters) generator() (basebuilder.Generator, error) {
	return builder.NewGenerator(builder.Config{
		RemoteSupportAddress: p.RemoteSupportAddress,
		RemoteSupportToken:   p.RemoteSupportToken,
		CACertPath:           p.CACertPath,
		EncryptionKey:        p.EncryptionKey,
	})
}

func (p buildParameters) builderConfig() (*basebuilder.Config, error) {
	config := p.BuilderConfig()
	var err error
	config.Generator, err = p.generator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.Repository = p.repository()
	config.Credentials = p.Credentials
	config.NewSyncer = builder.NewSyncer
	config.GetRepository = builder.GetRepository
	return &config, nil
}

func buildClusterImage(ctx context.Context, params buildParameters) error {
	config, err := params.builderConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterBuilder, err := basebuilder.NewClusterBuilder(*config)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterBuilder.Close()
	return clusterBuilder.Build(ctx, basebuilder.ClusterRequest{
		SourcePath: params.SourcePath,
		OutputPath: params.OutPath,
		Overwrite:  params.Overwrite,
		BaseImage:  params.BaseImage,
		Vendor:     params.Vendor,
	})
}
