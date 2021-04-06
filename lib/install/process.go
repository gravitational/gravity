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
	"fmt"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/users"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// InitProcess initializes and starts a gravity process
func InitProcess(ctx context.Context, gravityConfig processconfig.Config, newProcess process.NewGravityProcess) (process.GravityProcess, error) {
	teleportConfig := process.WizardTeleportConfig(gravityConfig.ClusterName, gravityConfig.DataDir)
	p, err := newProcess(ctx, gravityConfig, *teleportConfig)
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
	return p, nil
}

// NewProcessConfig creates a gravity process config from installer config
func NewProcessConfig(config ProcessConfig) (*processconfig.Config, error) {
	assetsDir := filepath.Join(config.WriteStateDir, "assets")
	if err := os.MkdirAll(assetsDir, defaults.SharedDirMask); err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err),
			"failed to create directory for assets")
	}
	healthAddr, _ := teleutils.ParseAddr(fmt.Sprintf(":%v", defaults.WizardHealthPort))
	adminRole, err := users.NewAdminRole()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wizardConfig := &processconfig.Config{
		Mode:         constants.ComponentInstaller,
		WebAssetsDir: assetsDir,
		DataDir:      config.WriteStateDir,
		Hostname:     config.AdvertiseAddr,
		HealthAddr:   *healthAddr,
		BackendType:  constants.BoltBackend,
		Pack: processconfig.PackageServiceConfig{
			ListenAddr: teleutils.NetAddr{
				AddrNetwork: "tcp",
				// Listen on all interfaces
				Addr: fmt.Sprintf("0.0.0.0:%v", defaults.WizardPackServerPort),
			},
			AdvertiseAddr: teleutils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        fmt.Sprintf("%v:%v", config.AdvertiseAddr, defaults.WizardPackServerPort),
			},
			ReadDir: config.StateDir,
		},
		Users: []processconfig.User{
			{
				Email:    defaults.WizardUser,
				Password: config.Token,
				Org:      defaults.SystemAccountOrg,
				Type:     storage.AdminUser,
				Roles:    []string{adminRole.GetName()},
			},
		},
		ServiceUser:     &config.ServiceUser,
		ClusterName:     config.ClusterName,
		Devmode:         config.Devmode,
		InstallLogFiles: []string{config.LogFile},
		InstallToken:    config.Token,
	}
	wizardConfig.OpsCenter.SeedConfig, err = process.RemoteAccessConfig(config.WriteStateDir)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if wizardConfig.OpsCenter.SeedConfig == nil {
		wizardConfig.OpsCenter.SeedConfig = &ops.SeedConfig{}
	}
	return wizardConfig, nil
}

// ProcessConfig defines the configuration for generating process configuration
type ProcessConfig struct {
	// AdvertiseAddr specifies the advertise address for the wizard process
	AdvertiseAddr string
	// StateDir specifies the read-only state directory for the wizard process
	StateDir string
	// WriteStateDir specifies the state directory for the wizard process
	WriteStateDir string
	// ServiceUser specifies the service user selected for the operation
	ServiceUser systeminfo.User
	// ClusterName specifies the name of the cluster to create
	ClusterName string
	// Devmode specifies whether the development mode is on
	Devmode bool
	// LogFile specifies the path to the operation log file
	LogFile string
	// Token specifies the token the wizard will use to authenticate joining agents.
	Token string
}

func shutdown(p process.GravityProcess) {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
	defer cancel()
	p.Shutdown(ctx)
}
