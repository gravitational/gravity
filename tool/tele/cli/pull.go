package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/gravitational/gravity/lib/hub"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

func pull(env localenv.LocalEnvironment, app, outFile string, force, quiet bool) error {
	fi, err := utils.StatFile(outFile)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if fi != nil && !force {
		return trace.AlreadyExists("file %v already exists, provide --force flag to overwrite it", outFile)
	}

	hub, err := hub.New(hub.Config{})
	if err != nil {
		return trace.Wrap(err)
	}

	locator, err := pack.MakeLocator(app)
	if err != nil {
		return trace.Wrap(err)
	}

	f, err := os.Create(outFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	progress := utils.NewProgress(context.TODO(), fmt.Sprintf("Application %v download", app), 1, quiet)
	defer progress.Stop()

	err = hub.Download(f, *locator, progress)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
