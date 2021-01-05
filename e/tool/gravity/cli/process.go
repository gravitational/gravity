package cli

import (
	"context"

	"github.com/gravitational/gravity/e/lib/process"
	ossprocess "github.com/gravitational/gravity/lib/process"
	gcfg "github.com/gravitational/gravity/lib/processconfig"

	tcfg "github.com/gravitational/teleport/lib/config"
)

func startProcess(configDir, importDir string) error {
	return ossprocess.Run(context.TODO(), configDir, importDir,
		func(ctx context.Context, gCfg gcfg.Config, tCfg tcfg.FileConfig) (ossprocess.GravityProcess, error) {
			return process.New(ctx, gCfg, tCfg)
		})
}
