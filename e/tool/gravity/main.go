package main

import (
	stdlog "log"
	"os"

	"github.com/gravitational/gravity/e/tool/gravity/cli"
	"github.com/gravitational/gravity/tool/common"

	teleutils "github.com/gravitational/teleport/lib/utils"
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
	app := kingpin.New("gravity", "Cluster management tool (enterprise version)")
	if err := run(app); err != nil {
		log.WithError(err).Warn("Command failed.")
		common.PrintError(err)
		os.Exit(255)
	}
}

func run(app *kingpin.Application) error {
	gravity := cli.RegisterCommands(app)
	return common.ProcessRunError(cli.Run(gravity))
}
