package main

import (
	stdlog "log"
	"os"

	"github.com/gravitational/gravity/tool/common"
	"github.com/gravitational/gravity/tool/gravity/cli"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	teleutils.InitLogger(teleutils.LoggingForCLI, log.InfoLevel)
	stdlog.SetOutput(log.StandardLogger().Writer())
	app := kingpin.New("gravity", "Cluster management tool")
	if err := run(app); err != nil {
		log.Error(trace.DebugReport(err))
		common.PrintError(err)
		os.Exit(255)
	}
}

func run(app *kingpin.Application) error {
	gravity := cli.RegisterCommands(app)
	return common.ProcessRunError(cli.Run(gravity))
}
