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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/tool/common"

	"github.com/gravitational/trace"
	"gopkg.in/alecthomas/kingpin.v2"
)

// LocalEnvironmentFactory defines an interface for creating operation-specific environments
type LocalEnvironmentFactory interface {
	// NewLocalEnv creates a new default environment
	NewLocalEnv() (*localenv.LocalEnvironment, error)
	// NewUpdateEnv creates a new environment for update operations
	NewUpdateEnv() (*localenv.LocalEnvironment, error)
	// NewJoinEnv creates a new environment for join operations
	NewJoinEnv() (*localenv.LocalEnvironment, error)
}

// LocalEnv returns an instance of a local environment for the specified
// command
func (g *Application) LocalEnv(cmd string) (*localenv.LocalEnvironment, error) {
	stateDir, err := g.stateDir(cmd)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return g.getEnv(stateDir)
}

// NewLocalEnv returns an instance of a local environment.
func (g *Application) NewLocalEnv() (*localenv.LocalEnvironment, error) {
	stateDir := *g.StateDir
	// most commands (with the exception of update or join/expand)
	// use the state directory set by original install/join command,
	// unless it was specified explicitly
	if stateDir == "" {
		dir, err := state.GetStateDir()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		stateDir = filepath.Join(dir, defaults.LocalDir)
	}
	return g.getEnv(stateDir)
}

// NewUpdateEnv returns an instance of the local environment that is used
// only for updates
func (g *Application) NewUpdateEnv() (*localenv.LocalEnvironment, error) {
	dir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return g.getEnv(state.GravityUpdateDir(dir))
}

// NewJoinEnv returns an instance of local environment where join-specific data is stored
func (g *Application) NewJoinEnv() (*localenv.LocalEnvironment, error) {
	err := os.MkdirAll(defaults.GravityJoinDir, defaults.SharedDirMask)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return g.getEnv(defaults.GravityJoinDir)
}

func (g *Application) getEnv(stateDir string) (*localenv.LocalEnvironment, error) {
	args := localenv.LocalEnvironmentArgs{
		StateDir:         stateDir,
		Insecure:         *g.Insecure,
		Silent:           localenv.Silent(*g.Silent),
		Debug:            *g.Debug,
		EtcdRetryTimeout: *g.EtcdRetryTimeout,
		Reporter:         common.ProgressReporter(*g.Silent),
	}
	if *g.StateDir != defaults.LocalGravityDir {
		args.LocalKeyStoreDir = *g.StateDir
	}
	// set insecure in devmode so we won't need to use
	// --insecure flag all the time
	cfg, _, err := processconfig.ReadConfig("")
	if err == nil && cfg.Devmode {
		args.Insecure = true
	}
	return localenv.NewLocalEnvironment(args)
}

// stateDir returns the local state directory for the specified command
func (g *Application) stateDir(cmd string) (string, error) {
	if g.isInstallCommand(cmd) || g.isJoinCommand(cmd) {
		// if a custom state directory was provided during install/join, it means
		// that user wants all gravity data to be stored under this directory
		if *g.StateDir != "" {
			err := state.SetStateDir(*g.StateDir)
			if err != nil {
				return "", trace.Wrap(err)
			}
			return filepath.Join(*g.StateDir, defaults.LocalDir), nil
		}
		// otherwise use default state dir
		return defaults.LocalGravityDir, nil
	}

	// all other commands should use the state directory that was set by original
	// install/join command, unless it was specified explicitly
	if *g.StateDir != "" {
		return *g.StateDir, nil
	}
	dir, err := state.GetStateDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(dir, defaults.LocalDir), nil
}

// isInstallCommand returns true if the specified command is
// a "gravity install" command
func (g *Application) isInstallCommand(cmd string) bool {
	switch cmd {
	case g.InstallCmd.FullCommand():
		return *g.InstallCmd.Phase == ""
	}
	return false
}

// isJoinCommand returns true if the specified command is
// a "gravity join" command
func (g *Application) isJoinCommand(cmd string) bool {
	switch cmd {
	case g.JoinCmd.FullCommand():
		return true
	}
	return false
}

func uninstallExistingAgentService() error {
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return svm.UninstallService(defaults.GravityRPCAgentServiceName)
}

func parseArgs(args []string) (*kingpin.ParseContext, error) {
	app := kingpin.New("gravity", "")
	app.Terminate(func(int) {})
	app.Writer(ioutil.Discard)
	return RegisterCommands(app).ParseContext(args)
}
