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

package process

import (
	"context"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// GravityProcess defines an interface for a process that runs gravity service
type GravityProcess interface {
	// Supervisor allows gravity process to register and manage internal services
	service.Supervisor
	// StartResumeOperationLoop starts service that resumes pending operations
	StartResumeOperationLoop()
	// ImportState imports gravity state from the specified directory
	ImportState(importDir string) error
	// InitRPCCredentials initializes package with RPC secrets
	InitRPCCredentials() error
	// AgentService returns the process agent service
	AgentService() ops.AgentService
	// UsersService returns the process identity service
	UsersService() users.Identity
	// Config returns the process config
	Config() *processconfig.Config
	// Shutdown starts graceful shutdown of the process,
	// blocks until all resources are freed and go-routines have shut down
	Shutdown(context.Context)
}

// NewGravityProcess defines a function that creates a gravity process
type NewGravityProcess func(ctx context.Context, gravityConfig processconfig.Config,
	teleportConfig config.FileConfig) (GravityProcess, error)

// NewProcess creates a new gravity API server process
//
// It satisfies NewGravityProcess function type.
func NewProcess(ctx context.Context, gravityConfig processconfig.Config, teleportConfig config.FileConfig) (GravityProcess, error) {
	return New(ctx, gravityConfig, teleportConfig)
}

// Run creates a new gravity process using the provided "constructor" function,
// launches it and waits for it to shut down
func Run(ctx context.Context, configDir, importDir string, newProcess NewGravityProcess) error {
	gravityConfig, teleportConfig, err := processconfig.ReadConfig(configDir)
	if err != nil {
		return trace.Wrap(err)
	}
	if gravityConfig.Devmode {
		logrus.SetLevel(logrus.DebugLevel)
	}
	gravityConfig.ImportDir = importDir
	process, err := newProcess(ctx, *gravityConfig, *teleportConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	err = process.Start()
	if err != nil {
		return trace.Wrap(err)
	}
	process.StartResumeOperationLoop()
	return process.Wait()
}
