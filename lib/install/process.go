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

	"github.com/gravitational/trace"
)

// InitProcess initializes and starts a gravity process
func InitProcess(ctx context.Context, installerConfig Config, gravityConfig processconfig.Config) (process.GravityProcess, error) {
	teleportConfig := process.WizardTeleportConfig(installerConfig.SiteDomain,
		installerConfig.WriteStateDir)
	p, err := installerConfig.NewProcess(ctx, gravityConfig, *teleportConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = InitLogging(installerConfig.SystemLogFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.InitRPCCredentials()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.Start()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = process.WaitForServiceStarted(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return p, nil
}

// MakeProcessConfig creates a gravity process config from installer config
func MakeProcessConfig(i Config) (*processconfig.Config, error) {
	cfg, err := process.WizardProcessConfig(i.AdvertiseAddr, i.StateDir, i.WriteStateDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.ServiceUser = &i.ServiceUser
	seedConfig, err := process.RemoteAccessConfig(i.StateDir)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if seedConfig != nil {
		cfg.OpsCenter.SeedConfig = seedConfig
	} else {
		cfg.OpsCenter.SeedConfig = &ops.SeedConfig{}
	}
	cfg.Mode = constants.ComponentInstaller
	cfg.ClusterName = i.SiteDomain
	cfg.Devmode = i.Insecure
	cfg.InstallLogFiles = append(cfg.InstallLogFiles, i.UserLogFile)
	return cfg, nil
}
