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

package install

import (
	"context"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/ops/opsclient"

	"github.com/gravitational/trace"
)

// Installer extends its open-source counterpart with additional features
// such as Ops Center support
type Installer struct {
	// Installer is the base installer
	*install.Installer
	// Config is the enterprise installer configuration
	Config Config
	// Operator is the enterprise ops service
	Operator ops.Operator
	// Remote is the remote Ops Center clients
	Remote *environment.Remote
	// downloadInstaller forces the application packages to be downloaded
	// prior to installation
	downloadInstaller bool
	// opsCenterCluster is the name of the Ops Center the cluster will
	// be connected to after installation
	opsCenterCluster string
}

// Config defines the installer configuration
type Config struct {
	// Config is the base installer configuration
	install.Config
	// License is optional license string
	License string
	// RemoteOpsURL is the URL of the remote Ops Center
	RemoteOpsURL string
	// RemoteOpsToken is the remote Ops Center auth token
	RemoteOpsToken string
	// OperationID is the ID of the operation when installing via Ops Center
	OperationID string
	// OpsTunnelToken is the token used when creating a trusted cluster
	OpsTunnelToken string
	// OpsSNIHost is the Ops Center SNI host
	OpsSNIHost string
	// OpsAdvertiseAddr is the Ops Center advertise address
	OpsAdvertiseAddr string
}

// CheckAndSetDefaults checks the parameters and sets some defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.RemoteOpsURL != "" && c.RemoteOpsToken == "" {
		return trace.BadParameter("missing RemoteOpsToken")
	}
	if c.Mode == constants.InstallModeOpsCenter {
		if c.RemoteOpsURL == "" {
			return trace.BadParameter("missing RemoteOpsURL")
		}
		if c.SiteDomain == "" {
			return trace.BadParameter("missing SiteDomain")
		}
		if c.Role == "" {
			return trace.BadParameter("missing Role")
		}
	}
	return nil
}

// Init creates and initializes a new installer
func Init(ctx context.Context, cfg Config) (*Installer, error) {
	// init installer from open-source first
	ossInstaller, err := install.Init(ctx, cfg.Config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ossOperator, ok := ossInstaller.Operator.(*opsclient.Client)
	if !ok {
		return nil, trace.BadParameter("unexpected type: %T", ossInstaller.Operator)
	}
	var operator ops.Operator = client.New(ossOperator)
	var remote *environment.Remote
	if cfg.RemoteOpsURL != "" {
		remote, err = environment.LoginRemote(cfg.RemoteOpsURL, cfg.RemoteOpsToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Mode == constants.InstallModeOpsCenter {
			operator = NewFanoutOperator(operator, remote.Operator)
		}
	}
	installer := &Installer{
		Installer: ossInstaller,
		Config:    cfg,
		Operator:  operator,
		Remote:    remote,
	}
	// set enterprise installer as engine to customize the installation flow
	installer.SetEngine(installer)
	// perform additional enterprise-specific bootstrap actions
	err = installer.bootstrap(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return installer, nil
}

// Start starts the installer in the appropriate mode according to its configuration
func (i *Installer) Start() error {
	// this install method is triggered when installing via an Ops Center
	if i.Mode == constants.InstallModeOpsCenter {
		i.PrintStep("Starting Ops Center initiated install")
		return i.StartOpsCenterInstall()
	}
	// none of enterprise-specific install methods matches, it must be
	// one of available in open-source then
	return i.Installer.Start()
}
