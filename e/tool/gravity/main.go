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

package main

import (
	stdlog "log"
	"os"

	"github.com/gravitational/gravity/e/tool/gravity/cli"
	installer "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	// the following modules are imported just so their init methods are called
	_ "github.com/gravitational/gravity/e/lib/catalog"
	_ "github.com/gravitational/gravity/e/lib/modules"
	_ "github.com/gravitational/gravity/e/lib/status"
)

func main() {
	teleutils.InitLogger(teleutils.LoggingForCLI, log.InfoLevel)
	stdlog.SetOutput(log.StandardLogger().Writer())
	app := kingpin.New("gravity", "Gravity cluster management tool (Enterprise edition).")
	if err := run(app); err != nil {
		if errCode, ok := trace.Unwrap(err).(utils.ExitCodeError); ok {
			if errCode != installer.ErrCompleted {
				log.WithError(err).Warn("Command failed.")
				common.PrintError(errCode.OrigError())
			}
			os.Exit(errCode.ExitCode())
		}
		log.WithError(err).Warn("Command failed.")
		common.PrintError(err)
		os.Exit(255)
	}
}

func run(app *kingpin.Application) error {
	gravity := cli.RegisterCommands(app)
	return common.ProcessRunError(cli.Run(gravity))
}
