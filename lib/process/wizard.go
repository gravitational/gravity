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
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"

	"github.com/gravitational/teleport/lib/backend"
	telecfg "github.com/gravitational/teleport/lib/config"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// WizardTeleportConfig generates a teleport configuration for the gravity wizard process
func WizardTeleportConfig(clusterName, stateDir string) *telecfg.FileConfig {
	return &telecfg.FileConfig{
		Global: telecfg.Global{
			DataDir: stateDir,
			Storage: backend.Config{
				// TODO Eventually we should change this to "dir" backend
				// because bolt backend is being deprecated in teleport
				Type:   "bolt",
				Params: make(backend.Params),
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
			ClusterName: telecfg.ClusterName(constants.InstallerClusterName(clusterName)),
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
