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

package install

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/systeminfo"

	"github.com/gravitational/trace"
)

// InitProcess initializes and starts a gravity process
func InitProcess(ctx context.Context, gravityConfig processconfig.Config, newProcess process.NewGravityProcess) (process.GravityProcess, error) {
	teleportConfig := process.WizardTeleportConfig(gravityConfig.ClusterName, gravityConfig.DataDir)
	p, err := newProcess(ctx, gravityConfig, *teleportConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	setLoggingOptions()
	err = p.InitRPCCredentials()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.Start()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = process.WaitForServiceStarted(ctx, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return p, nil
}

// NewProcessConfig creates a gravity process config from installer config
func NewProcessConfig(config ProcessConfig) (*processconfig.Config, error) {
	wizardConfig, err := process.WizardProcessConfig(config.AdvertiseAddr, config.StateDir, config.WriteStateDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wizardConfig.ServiceUser = &config.ServiceUser
	seedConfig, err := process.RemoteAccessConfig(config.StateDir)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if seedConfig != nil {
		wizardConfig.OpsCenter.SeedConfig = seedConfig
	} else {
		wizardConfig.OpsCenter.SeedConfig = &ops.SeedConfig{}
	}
	wizardConfig.Mode = constants.ComponentInstaller
	wizardConfig.ClusterName = config.ClusterName
	wizardConfig.Devmode = config.Devmode
	wizardConfig.InstallLogFiles = append(wizardConfig.InstallLogFiles, config.LogFile)
	return wizardConfig, nil
}

// ProcessConfig defines the configuration for generating process configuration
type ProcessConfig struct {
	Hostname      string
	AdvertiseAddr string
	StateDir      string
	WriteStateDir string
	ServiceUser   systeminfo.User
	ClusterName   string
	Devmode       bool
	LogFile       string
}
