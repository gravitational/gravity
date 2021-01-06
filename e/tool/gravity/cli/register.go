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
	"github.com/gravitational/gravity/e/lib/defaults"
	"github.com/gravitational/gravity/tool/common"
	"github.com/gravitational/gravity/tool/gravity/cli"

	"gopkg.in/alecthomas/kingpin.v2"
)

func RegisterCommands(app *kingpin.Application) *Application {
	g := &Application{
		// register all commands from open-source
		Application: cli.RegisterCommands(app),
	}

	// register additional enterprise-specific commands, or extend open-source
	// commands with extra args/flags

	g.InstallCmd.InstallCmd = &g.Application.InstallCmd
	g.InstallCmd.License = g.InstallCmd.Flag("license", "Application license, optional").String()
	g.InstallCmd.OpsAdvertiseAddr = g.InstallCmd.Flag("ops-advertise-addr", "Ops center advertise address 'example.com:<web-port>'").String()
	g.InstallCmd.OperationID = g.InstallCmd.Flag("operation-id", "Operation ID when installing via Ops Center").Hidden().String()
	g.InstallCmd.OpsCenterURL = g.InstallCmd.Flag("ops-url", "URL of the Ops Center to connect to").Hidden().String()
	g.InstallCmd.OpsCenterToken = g.InstallCmd.Flag("ops-token", "Auth token for the Ops Center specified with --ops-url flag").Hidden().String()
	g.InstallCmd.OpsCenterTunnelToken = g.InstallCmd.Flag("ops-tunnel-token", "Trusted cluster token").Hidden().String()
	g.InstallCmd.OpsCenterSNIHost = g.InstallCmd.Flag("ops-sni-host", "Public advertise hostname of the Ops Center").Hidden().String()

	g.StatusCmd.StatusCmd = &g.Application.StatusCmd
	g.StatusCmd.Tunnel = g.StatusCmd.Flag("tunnel", "Show only the remote assistance status").Bool()

	g.UpdateDownloadCmd.CmdClause = g.UpdateCmd.Command("download", "Check for and download newer version of the running application").Hidden()
	g.UpdateDownloadCmd.Every = g.UpdateDownloadCmd.Flag("every", "Enable automatic downloading of the updates with the specified interval").String()

	g.OpsGenerateCmd.CmdClause = g.OpsCmd.Command("create-wizard", "Generate a standalone installer for an application").Hidden()
	g.OpsGenerateCmd.Package = cli.Locator(g.OpsGenerateCmd.Arg("package", "The application locator").Required())
	g.OpsGenerateCmd.Dir = g.OpsGenerateCmd.Arg("dir", "Directory where installer files will be written to").Required().String()
	g.OpsGenerateCmd.CACert = g.OpsGenerateCmd.Flag("ca-cert", "Path to CA certificate file; if not provided, the Ops Center's CA will be used").String()
	g.OpsGenerateCmd.EncryptionKey = g.OpsGenerateCmd.Flag("encryption-key", "Optional key to encrypt installer packages with").String()
	g.OpsGenerateCmd.OpsCenterURL = g.OpsGenerateCmd.Flag("ops-url", "URL of the Ops Center to use for installer generation").String()

	g.TunnelCmd.CmdClause = g.Command("tunnel", "Configure remote access to Ops Center")
	g.TunnelEnableCmd.CmdClause = g.TunnelCmd.Command("enable", "Enable remote access to the Ops Center")
	g.TunnelDisableCmd.CmdClause = g.TunnelCmd.Command("disable", "Disable remote access to the Ops Center")
	g.TunnelStatusCmd.CmdClause = g.TunnelCmd.Command("status", "Check status of the connection to the Ops Center")

	g.LicenseCmd.CmdClause = g.Command("license", "Operations with cluster licenses").Hidden()

	g.LicenseInstallCmd.CmdClause = g.LicenseCmd.Command("install", "Install (or update) a cluster license").Hidden()
	g.LicenseInstallCmd.Path = g.LicenseInstallCmd.Flag("from-file", "Path to the license file").Required().String()

	g.LicenseNewCmd.CmdClause = g.LicenseCmd.Command("new", "Generate a new license").Hidden()
	g.LicenseNewCmd.MaxNodes = g.LicenseNewCmd.Flag("max-nodes", "Maximum amount of nodes").Required().Int()
	g.LicenseNewCmd.ValidFor = g.LicenseNewCmd.Flag("valid-for", "Validity duration in Go duration format").Required().String()
	g.LicenseNewCmd.StopApp = g.LicenseNewCmd.Flag("stop-app", "If provided, the app will be stopped once license expires").Bool()
	g.LicenseNewCmd.CACert = g.LicenseNewCmd.Flag("ca-cert", "Path to CA certificate file").Required().String()
	g.LicenseNewCmd.CAKey = g.LicenseNewCmd.Flag("ca-key", "Path to CA private key file").Required().String()
	g.LicenseNewCmd.EncryptionKey = g.LicenseNewCmd.Flag("encryption-key", "Hex encoded encryption key").String()
	g.LicenseNewCmd.CustomerName = g.LicenseNewCmd.Flag("customer-name", "Name of the customer to generate license for").String()
	g.LicenseNewCmd.CustomerEmail = g.LicenseNewCmd.Flag("customer-email", "Email of the customer to generate license for").String()
	g.LicenseNewCmd.CustomerMetadata = g.LicenseNewCmd.Flag("customer-metadata", "Custom metadata to attach to license").String()
	g.LicenseNewCmd.ProductName = g.LicenseNewCmd.Flag("product-name", "Name of the product to generate license for").String()
	g.LicenseNewCmd.ProductVersion = g.LicenseNewCmd.Flag("product-version", "Version of the product to generate license for").String()

	g.LicenseShowCmd.CmdClause = g.LicenseCmd.Command("show", "Show the cluster license").Hidden()
	g.LicenseShowCmd.Output = common.Format(g.LicenseShowCmd.Flag("output", "Output format: pem or json").Default(string(defaults.LicenseOutputFormat)))

	return g
}
