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

package builder

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/builder"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// generator generates the enteprise installer
type generator struct {
	// Config is the generator configuration
	Config
}

// Config is the generator configuration
type Config struct {
	// RemoteSupport is an address of an Ops Center clusters should connect to
	// after installation
	RemoteSupportAddress string
	// RemoteSupportToken is the authentication token used to connector to
	// an Ops Center
	RemoteSupportToken string
	// CACertPath is a path to the CA certificate to pack with the installer
	CACertPath string
	// caCert is the CA certificate to pack with the installer
	caCert []byte
	// EncryptionKey is a key used to encrypt packages at rest in the installer
	EncryptionKey string
}

// CheckAndSetDefaults validates generator config and fills in defaults
func (c *Config) CheckAndSetDefaults() (err error) {
	if c.RemoteSupportAddress != "" && c.RemoteSupportToken == "" {
		return trace.BadParameter("remote support token is not provided")
	}
	if c.RemoteSupportAddress == "" && c.RemoteSupportToken != "" {
		return trace.BadParameter("remote support address is not provided")
	}
	if c.CACertPath != "" {
		c.caCert, err = ioutil.ReadFile(c.CACertPath)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// NewGenerator creates a new generator instance based on the provided configuration
func NewGenerator(config Config) (*generator, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &generator{
		Config: config,
	}, nil
}

// Generate extends the installer generation logic from open-source by
// considering additional features such as remote support and package
// encyption
func (g *generator) Generate(builder *builder.Engine, manifest *schema.Manifest, application app.Application) (io.ReadCloser, error) {
	installerReq := app.InstallerRequest{
		Application:   application.Package,
		CACert:        string(g.caCert),
		EncryptionKey: g.EncryptionKey,
	}
	// if remote support options are present, the installed cluster will connect
	// to the specified Ops Center
	if g.RemoteSupportAddress != "" {
		err := g.configureTrustedClusters(builder, &installerReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	// if there are any extensions, configure them for the installer too
	if manifest.Extensions != nil {
		err := g.configureExtensions(manifest, &installerReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return builder.Apps.GetAppInstaller(installerReq)
}

func (g *generator) configureTrustedClusters(builder *builder.Engine, req *app.InstallerRequest) error {
	account, err := g.createSystemAccount(builder)
	if err != nil {
		return trace.Wrap(err)
	}
	req.Account = *account
	opsCenterURL, err := url.ParseRequestURI(g.RemoteSupportAddress)
	if err != nil {
		return trace.Wrap(err)
	}
	opsCenterHostname, _, err := net.SplitHostPort(opsCenterURL.Host)
	if err != nil {
		return trace.Wrap(err)
	}
	req.TrustedCluster = storage.NewTrustedCluster(opsCenterHostname,
		storage.TrustedClusterSpecV2{
			Enabled:              true,
			Token:                g.RemoteSupportToken,
			ProxyAddress:         fmt.Sprintf("%v:%v", opsCenterHostname, defaults.HTTPSPort),
			ReverseTunnelAddress: fmt.Sprintf("%v:%v", opsCenterHostname, teledefaults.SSHProxyTunnelListenPort),
			PullUpdates:          true,
		})
	return nil
}

func (g *generator) createSystemAccount(builder *builder.Engine) (*storage.Account, error) {
	return builder.Backend.CreateAccount(storage.Account{
		ID:  defaults.SystemAccountID,
		Org: defaults.SystemAccountOrg,
	})
}

func (g *generator) configureExtensions(manifest *schema.Manifest, req *app.InstallerRequest) error {
	if manifest.Extensions.Encryption != nil {
		req.EncryptionKey = manifest.Extensions.Encryption.EncryptionKey
		req.CACert = manifest.Extensions.Encryption.CACert
	}
	return nil
}

// NewSyncer returns a new syncer instance for the provided builder
func NewSyncer(b *builder.Engine) (builder.Syncer, error) {
	repository, err := b.GetRepository(b)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pack, err := b.Env.PackageService(repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apps, err := b.Env.AppService(repository, localenv.AppConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return builder.NewPackSyncer(pack, apps, repository), nil
}

// GetRepository returns package source repository for the provided builder
func GetRepository(b *builder.Engine) (string, error) {
	// use repository set explicitly on the CLI (via --repository flag)
	if b.Repository != "" {
		return b.Repository, nil
	}
	// if it wasn't set, look for an cluster we're logged into
	credentials, err := b.CredentialsService.Current()
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}
	// otherwise use the default one
	if trace.IsNotFound(err) {
		return defaults.DistributionOpsCenter, nil
	}
	return credentials.URL, nil
}
