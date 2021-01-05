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
	g.InstallCmd.License = g.InstallCmd.Flag("license", "Cluster image license, in PEM format.").String()
	g.InstallCmd.LicenseFile = g.InstallCmd.Flag("license-file", "Path to the cluster license file in PEM format.").String()
	g.InstallCmd.OpsAdvertiseAddr = g.InstallCmd.Flag("ops-advertise-addr", `[Obsolete] Gravity Hub advertise address, e.g. "ops.example.com:<web-port>".`).Hidden().String()
	g.InstallCmd.HubAdvertiseAddr = g.InstallCmd.Flag("hub-advertise-addr", `Gravity Hub advertise address, e.g. "hub.example.com:<web-port>".`).String()
	g.InstallCmd.OperationID = g.InstallCmd.Flag("operation-id", "Operation ID when installing via Gravity Hub.").Hidden().String()
	g.InstallCmd.OpsCenterURL = g.InstallCmd.Flag("ops-url", "URL of the Gravity Hub to connect to.").Hidden().String()
	g.InstallCmd.OpsCenterToken = g.InstallCmd.Flag("ops-token", "Auth token for the Gravity Hub specified with --ops-url flag.").Hidden().String()
	g.InstallCmd.OpsCenterTunnelToken = g.InstallCmd.Flag("ops-tunnel-token", "Trusted cluster token.").Hidden().String()
	g.InstallCmd.OpsCenterSNIHost = g.InstallCmd.Flag("ops-sni-host", "Public advertise hostname of the Gravity Hub.").Hidden().String()

	g.StatusCmd.StatusCmd = &g.Application.StatusCmd
	g.StatusCmd.Tunnel = g.StatusCmd.Flag("tunnel", "Show only the remote assistance status.").Bool()

	g.UpdateDownloadCmd.CmdClause = g.UpdateCmd.Command("download", "Check for and download newer version of the cluster and application images.").Hidden()
	g.UpdateDownloadCmd.Every = g.UpdateDownloadCmd.Flag("every", "Enable automatic downloading of new versions at the specified interval.").String()

	g.OpsGenerateCmd.CmdClause = g.OpsCmd.Command("create-wizard", "Generate a standalone installer for an application").Hidden()
	g.OpsGenerateCmd.Package = cli.Locator(g.OpsGenerateCmd.Arg("package", "The application locator").Required())
	g.OpsGenerateCmd.Dir = g.OpsGenerateCmd.Arg("dir", "Directory where installer files will be written to").Required().String()
	g.OpsGenerateCmd.CACert = g.OpsGenerateCmd.Flag("ca-cert", "Path to CA certificate file; if not provided, the Gravity Hub's CA will be used").String()
	g.OpsGenerateCmd.EncryptionKey = g.OpsGenerateCmd.Flag("encryption-key", "Optional key to encrypt installer packages with").String()
	g.OpsGenerateCmd.OpsCenterURL = g.OpsGenerateCmd.Flag("ops-url", "URL of the Gravity Hub to use for installer generation").String()

	g.TunnelCmd.CmdClause = g.Command("tunnel", "Configure remote access to Gravity Hub.")
	g.TunnelEnableCmd.CmdClause = g.TunnelCmd.Command("enable", "Enable remote access to the Gravity Hub.")
	g.TunnelDisableCmd.CmdClause = g.TunnelCmd.Command("disable", "Disable remote access to the Gravity Hub.")
	g.TunnelStatusCmd.CmdClause = g.TunnelCmd.Command("status", "Check status of the connection to the Gravity Hub.")

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
