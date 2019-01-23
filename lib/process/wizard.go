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
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/users"

	telecfg "github.com/gravitational/teleport/lib/config"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// WizardProcessConfig returns new process config in wizard mode
func WizardProcessConfig(hostname, readStateDir, stateDir string) (*processconfig.Config, error) {
	assetsDir, err := ioutil.TempDir("", "assets")
	if err != nil {
		return nil, trace.Wrap(err, "failed to create temporary directory for assets")
	}
	healthAddr, _ := teleutils.ParseAddr(fmt.Sprintf(":%v", defaults.WizardHealthPort))
	adminRole, err := users.NewAdminRole()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &processconfig.Config{
		Mode:         constants.ComponentInstaller,
		WebAssetsDir: assetsDir,
		DataDir:      stateDir,
		Hostname:     hostname,
		HealthAddr:   *healthAddr,
		BackendType:  constants.BoltBackend,
		OpsCenter:    processconfig.OpsCenterConfig{},
		Pack: processconfig.PackageServiceConfig{
			ListenAddr: teleutils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        fmt.Sprintf(":%v", defaults.WizardPackServerPort),
			},
			AdvertiseAddr: teleutils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        fmt.Sprintf("%v:%v", hostname, defaults.WizardPackServerPort),
			},
			ReadDir: readStateDir,
		},
		Users: []processconfig.User{
			{
				Email:    defaults.WizardUser,
				Password: defaults.WizardPassword,
				Org:      defaults.SystemAccountOrg,
				Type:     storage.AdminUser,
				Roles:    []string{adminRole.GetName()},
			},
		},
	}, nil
}

// WizardTeleportConfig generates a teleport configuration for the gravity wizard process
func WizardTeleportConfig(clusterName, stateDir string) *telecfg.FileConfig {
	return &telecfg.FileConfig{
		Global: telecfg.Global{
			DataDir: stateDir,
			Logger: telecfg.Log{
				Output:   "stderr",
				Severity: "warn",
			},
			AuthServers:   []string{fmt.Sprintf("localhost:%v", defaults.WizardAuthServerPort)},
			Ciphers:       defaults.TeleportCiphers,
			KEXAlgorithms: defaults.TeleportKEXAlgorithms,
			MACAlgorithms: defaults.TeleportMACAlgorithms,
		},
		Auth: telecfg.Auth{
			Service: telecfg.Service{
				EnabledFlag:   "yes",
				ListenAddress: fmt.Sprintf("localhost:%v", defaults.WizardAuthServerPort),
			},
			ClusterName: telecfg.ClusterName(fmt.Sprintf("%v%v",
				constants.InstallerTunnelPrefix, clusterName)),
		},
		Proxy: telecfg.Proxy{
			Service: telecfg.Service{
				EnabledFlag:   "yes",
				ListenAddress: fmt.Sprintf("0.0.0.0:%v", defaults.WizardProxyServerPort),
			},
			TunAddr: fmt.Sprintf("0.0.0.0:%v", defaults.WizardReverseTunnelPort),
			WebAddr: fmt.Sprintf("0.0.0.0:%v", defaults.WizardWebProxyPort),
		},
		SSH: telecfg.SSH{
			Service: telecfg.Service{
				ListenAddress: fmt.Sprintf("0.0.0.0:%v", defaults.WizardSSHServerPort),
			},
		},
	}
}

// RemoteAccessConfig returns remote access configuration provided during
// the build of this package
func RemoteAccessConfig(stateDir string) (seedConfig *ops.SeedConfig, err error) {
	backend, err := keyval.NewBolt(keyval.BoltConfig{
		Path:     filepath.Join(stateDir, defaults.GravityDBFile),
		Readonly: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer backend.Close()
	clusters, err := backend.GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accounts, err := backend.GetAccounts()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(accounts) == 0 {
		return nil, trace.NotFound("no accounts found")
	}
	return &ops.SeedConfig{
		Account:         &accounts[0],
		TrustedClusters: convertClusters(clusters),
	}, nil
}

// convertClusters is a helper to convert Teleport's trusted clusters to
// the Telekube's interface
func convertClusters(clusters []teleservices.TrustedCluster) (result []storage.TrustedCluster) {
	for _, cluster := range clusters {
		if c, ok := cluster.(storage.TrustedCluster); ok {
			result = append(result, c)
		}
	}
	return result
}
