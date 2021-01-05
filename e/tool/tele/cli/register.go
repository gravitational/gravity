package cli

import (
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/tool/common"
	"github.com/gravitational/gravity/tool/tele/cli"

	"gopkg.in/alecthomas/kingpin.v2"
)

// RegisterCommands registers all tele tool flags, arguments and subcommands
func RegisterCommands(app *kingpin.Application) Application {
	t := Application{
		Application: cli.RegisterCommands(app),
	}

	supportedResources := modules.GetResources().SupportedResources()
	supportedResourcesToRemove := modules.GetResources().SupportedResourcesToRemove()

	t.Hub = app.Flag("hub", "Address of the Gravity Hub to execute the command against. Defaults to the currently active cluster or the Gravitational distribution portal.").String()
	t.Token = app.Flag("token", "Gravity authentication token.").Short('t').String()

	t.BuildCmd.BuildCmd = &t.Application.BuildCmd
	t.BuildCmd.RemoteSupport = t.BuildCmd.Flag("remote-support-addr", "Address of the Gravity Hub installed clusters should connect to.").String()
	t.BuildCmd.RemoteSupportToken = t.BuildCmd.Flag("remote-support-token", "Token for connecting to the Gravity Hub.").String()
	t.BuildCmd.CACert = t.BuildCmd.Flag("ca-cert", "Path to the CA certificate file to use when building a cluster image.").String()
	t.BuildCmd.EncryptionKey = t.BuildCmd.Flag("encryption-key", "Encryption key to encrypt cluster image packages with.").String()
	t.BuildCmd.Repository = t.BuildCmd.Flag("repository", "[DEPRECATED, replaced by --hub] Optional address of Gravity Hub to download dependencies from.").Hidden().String()

	t.LoginCmd.CmdClause = app.Command("login", "[DEPRECATED. Use tsh login.] Log into Gravity Hub.").Hidden()
	t.LoginCmd.Cluster = t.LoginCmd.Arg("cluster", "Cluster name to log into.").String()
	t.LoginCmd.OpsCenter = t.LoginCmd.Flag("ops", "Gravity Hub address to log into.").Short('o').Hidden().String()
	t.LoginCmd.ConnectorID = t.LoginCmd.Flag("auth", "Authentication connector name to use.").String()
	t.LoginCmd.TTL = t.LoginCmd.Flag("ttl", fmt.Sprintf("Set authentication expiry time. Max is %v.", constants.MaxInteractiveSessionTTL)).Default(constants.MaxInteractiveSessionTTL.String()).Duration()

	t.LogoutCmd.CmdClause = app.Command("logout", "[DEPRECATED. Use tsh logout.] Log out of Gravity Hub.").Hidden()

	t.StatusCmd.CmdClause = app.Command("status", "[DEPRECATED. Use tsh status.] Print current login information.").Hidden()

	t.PushCmd.CmdClause = app.Command("push", "Push a cluster or application image to Gravity Hub.")
	t.PushCmd.Tarball = t.PushCmd.Arg("path", "Path to a cluster or application image file.").Required().String()
	t.PushCmd.Force = t.PushCmd.Flag("force", "Overwrite the existing image in the Gravity Hub.").Short('f').Bool()
	t.PushCmd.Quiet = t.PushCmd.Flag("quiet", "Suppress any output to stdout.").Short('q').Bool()

	t.CreateCmd.CmdClause = app.Command("create", fmt.Sprintf("Create or update a configuration resource, e.g. 'tele create cluster.yaml'. Supported resources are: %v.",
		supportedResources))
	t.CreateCmd.Filename = t.CreateCmd.Arg("filename", "Resource definition file.").String()
	t.CreateCmd.Force = t.CreateCmd.Flag("force", "Overwrite the resource if it already exists.").Short('f').Bool()

	t.GetCmd.CmdClause = app.Command("get", fmt.Sprintf("Get configuration resources, e.g. 'tele get clusters'. Supported resources are: %v.",
		supportedResources))
	t.GetCmd.Kind = t.GetCmd.Arg("kind", fmt.Sprintf("Resource kind. One of: %v.", supportedResources)).Required().String()
	t.GetCmd.Name = t.GetCmd.Arg("name", "Optional resource name.").String()
	t.GetCmd.Format = common.Format(t.GetCmd.Flag("format", "Output format: text, json or yaml.").Hidden().Default(string(constants.EncodingText)))
	t.GetCmd.Output = common.Format(t.GetCmd.Flag("output", "Output format: text, json or yaml.").Short('o'))

	t.RemoveCmd.CmdClause = app.Command("rm", fmt.Sprintf("Remove a configuration resource, e.g. 'tele rm cluster test'. Supported resources are: %v.",
		supportedResourcesToRemove))
	t.RemoveCmd.Kind = t.RemoveCmd.Arg("kind", fmt.Sprintf("Resource kind. One of: %v.", supportedResourcesToRemove)).Required().String()
	t.RemoveCmd.Name = t.RemoveCmd.Arg("name", "Resource name.").Required().String()
	t.RemoveCmd.Force = t.RemoveCmd.Flag("force", "Suppress not found errors.").Short('f').Bool()

	return t
}
