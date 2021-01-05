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
	"fmt"

	"github.com/gravitational/gravity/e/lib/ops/resources/tele"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/tool/common"
	"github.com/gravitational/gravity/tool/tele/cli"

	"gopkg.in/alecthomas/kingpin.v2"
)

// RegisterCommands registers all tele tool flags, arguments and subcommands
func RegisterCommands(app *kingpin.Application) Application {
	t := Application{
		Application: cli.RegisterCommands(app),
	}

	t.BuildCmd.BuildCmd = &t.Application.BuildCmd
	t.BuildCmd.RemoteSupport = t.BuildCmd.Flag("remote-support-addr", "Address of remote Ops Center installed clusters should connect to").String()
	t.BuildCmd.RemoteSupportToken = t.BuildCmd.Flag("remote-support-token", "Token for connecting to remote Ops Center").String()
	t.BuildCmd.CACert = t.BuildCmd.Flag("ca-cert", "Path to CA certificate file to bundle with the installer").String()
	t.BuildCmd.EncryptionKey = t.BuildCmd.Flag("encryption-key", "Key to encrypt installer packages with").String()

	t.LoginCmd.CmdClause = app.Command("login", "Login into Ops Center")
	t.LoginCmd.Cluster = t.LoginCmd.Arg("cluster", "Cluster to login into").String()
	t.LoginCmd.OpsCenter = t.LoginCmd.Flag("ops", "Ops Center to login into").Short('o').String()
	t.LoginCmd.ConnectorID = t.LoginCmd.Flag("auth", "Authentication connector name to use").String()
	t.LoginCmd.TTL = t.LoginCmd.Flag("ttl", fmt.Sprintf("Set authentication expiry time, max is %v", constants.MaxInteractiveSessionTTL)).Default(constants.MaxInteractiveSessionTTL.String()).Duration()
	t.LoginCmd.Token = t.LoginCmd.Flag("token", "Token to log into Ops Center with").String()

	t.LogoutCmd.CmdClause = app.Command("logout", "Logout from Ops Center")

	t.StatusCmd.CmdClause = app.Command("status", "Print login information")

	t.PushCmd.CmdClause = app.Command("push", "Push an application to remote Ops Center")
	t.PushCmd.Tarball = t.PushCmd.Arg("tarball", "Path to application tarball built with 'tele build' to upload to remote Ops Center").Required().String()
	t.PushCmd.Force = t.PushCmd.Flag("force", "Replace existing application").Short('f').Bool()
	t.PushCmd.Quiet = t.PushCmd.Flag("quiet", "Suppress any extra output to stdout").Short('q').Bool()

	t.CreateCmd.CmdClause = app.Command("create", fmt.Sprintf("Create or update a configuration resource, e.g. 'tele create cluster.yaml'. Supported resources are: %v", tele.SupportedResources))
	t.CreateCmd.Filename = t.CreateCmd.Arg("filename", "resource definition file").String()
	t.CreateCmd.Force = t.CreateCmd.Flag("force", "Overwrites a resource if it already exists. (update)").Short('f').Bool()

	t.GetCmd.CmdClause = app.Command("get", fmt.Sprintf("Get configuration resources, e.g. 'tele get clusters'. Supported resources are: %v", tele.SupportedResources))
	t.GetCmd.Kind = t.GetCmd.Arg("kind", fmt.Sprintf("resource kind, one of %v", tele.SupportedResources)).Required().String()
	t.GetCmd.Name = t.GetCmd.Arg("name", "optional resource name").String()
	t.GetCmd.Format = common.Format(t.GetCmd.Flag("format", "resource format, e.g. 'text', 'json' or 'yaml'").Default(string(constants.EncodingText)))
	t.GetCmd.Output = common.Format(t.GetCmd.Flag("output", "output format, e.g. 'text', 'json' or 'yaml'").Short('o'))

	t.RemoveCmd.CmdClause = app.Command("rm", fmt.Sprintf("Remove a configuration resource, e.g. 'tele rm cluster test'. Supported resources are: %v", tele.SupportedResources))
	t.RemoveCmd.Kind = t.RemoveCmd.Arg("kind", fmt.Sprintf("resource kind, one of %v", tele.SupportedResources)).Required().String()
	t.RemoveCmd.Name = t.RemoveCmd.Arg("name", "resource name, e.g. github").Required().String()
	t.RemoveCmd.Force = t.RemoveCmd.Flag("force", "Do not return errors if a resource is not found").Short('f').Bool()

	return t
}
