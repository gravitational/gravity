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
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/tool/common"

	"gopkg.in/alecthomas/kingpin.v2"
)

// RegisterCommands registers all tele tool flags, arguments and subcommands
func RegisterCommands(app *kingpin.Application) Application {
	tele := Application{
		Application: app,
	}

	tele.Debug = app.Flag("debug", "Enable debug mode").Bool()
	tele.Insecure = app.Flag("insecure", "Skip TLS verification when making HTTP requests").Default("false").Bool()
	tele.StateDir = app.Flag("state-dir", "Directory for temporary local state").Hidden().String()

	tele.VersionCmd.CmdClause = app.Command("version", "Print version and exit")
	tele.VersionCmd.Output = common.Format(tele.VersionCmd.Flag("output", "Output format, text or json").Short('o').Default(string(constants.EncodingText)))

	tele.BuildCmd.CmdClause = app.Command("build", "Build an application installer")
	tele.BuildCmd.ManifestPath = tele.BuildCmd.Arg("manifest-path", fmt.Sprintf("Path to the application manifest file, must be %q", defaults.ManifestFileName)).Default(defaults.ManifestFileName).String()
	tele.BuildCmd.OutFile = tele.BuildCmd.Flag("output", "Name of the generated tarball, defaults to <dirname>.tar.gz where <dirname> is the name of the directory where app manifest is located").Short('o').String()
	tele.BuildCmd.Overwrite = tele.BuildCmd.Flag("overwrite", "Overwrite the existing tarball").Short('f').Bool()
	tele.BuildCmd.Repository = tele.BuildCmd.Flag("repository", "Optional address of Ops Center to download dependencies from").Hidden().String()
	tele.BuildCmd.Name = tele.BuildCmd.Flag("name", "Optional application name, overrides the one specified in the manifest file").Hidden().String()
	tele.BuildCmd.Version = tele.BuildCmd.Flag("version", "Optional application version, overrides the one specified in the manifest file").Hidden().String()
	tele.BuildCmd.VendorPatterns = tele.BuildCmd.Flag("glob", "File pattern to search for container image references").Default(defaults.VendorPattern).Hidden().Strings()
	tele.BuildCmd.VendorIgnorePatterns = tele.BuildCmd.Flag("ignore", "Ignore files matching this regular expression when searching for container references").Hidden().Strings()
	tele.BuildCmd.SetImages = loc.ImagesSlice(tele.BuildCmd.Flag("set-image", "Rewrite docker image versions in the application resource files during vendoring, e.g. 'postgres:9.3.4' will rewrite all images with name 'postgres' to 'postgres:9.3.4'").Hidden())
	tele.BuildCmd.SetDeps = loc.LocatorSlice(tele.BuildCmd.Flag("set-dep", "Rewrite dependencies section in the application manifest file during vendoring, e.g. 'gravitational.io/site-app:0.0.39' will overwrite dependency to 'gravitational.io/site-app:0.0.39'").Hidden())
	tele.BuildCmd.SkipVersionCheck = tele.BuildCmd.Flag("skip-version-check", "Skip version compatibility check").Hidden().Bool()
	tele.BuildCmd.Parallel = tele.BuildCmd.Flag("parallel", "Specifies the number of concurrent tasks. If < 0, the number of tasks is not restricted, if unspecified, then tasks are capped at the number of logical CPU cores").Int()
	tele.BuildCmd.Quiet = tele.BuildCmd.Flag("quiet", "Suppress any extra output to stdout").Short('q').Bool()

	tele.ListCmd.CmdClause = app.Command("ls", "Display a list of user applications published in remote Ops Center")
	tele.ListCmd.Runtimes = tele.ListCmd.Flag("runtimes", "Show only runtimes").Short('r').Hidden().Bool()
	tele.ListCmd.Format = common.Format(tele.ListCmd.Flag("format", fmt.Sprintf("Output format, one of: %v", constants.OutputFormats)).Default(string(constants.EncodingText)))
	tele.ListCmd.All = tele.ListCmd.Flag("all", "Display all available versions").Bool()

	tele.PullCmd.CmdClause = app.Command("pull", "Pull an application from remote Ops Center")
	tele.PullCmd.App = tele.PullCmd.Arg("app", "Name of application to download: <name>:<version> or just <name> to download the latest").Required().String()
	tele.PullCmd.OutFile = tele.PullCmd.Flag("output", "Name of downloaded tarball, defaults to <name>-<version>.tar").Short('o').String()
	tele.PullCmd.Force = tele.PullCmd.Flag("force", "Overwrite existing tarball").Short('f').Bool()
	tele.PullCmd.Quiet = tele.PullCmd.Flag("quiet", "Suppress any extra output to stdout").Short('q').Bool()

	return tele
}
