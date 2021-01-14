// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import (
	ossinstall "github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// NewProcessConfig creates a gravity process config from installer config
func NewProcessConfig(config ProcessConfig) (*processconfig.Config, error) {
	// first make the open-source config
	gravityConfig, err := ossinstall.NewProcessConfig(config.ProcessConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// now extend it with enterprise-specific values
	if config.OpsAdvertiseAddr != "" {
		// in case of Ops Center install, its SNI host is the advertise hostname
		gravityConfig.OpsCenter.SeedConfig.SNIHost, _ = utils.SplitHostPort(config.OpsAdvertiseAddr, "")
	} else {
		// in case of regular cluster install, the Ops Center SNI host might
		// have been provided on the CLI (e.g. by install instructions
		// generated by the Ops Center)
		gravityConfig.OpsCenter.SeedConfig.SNIHost = config.OpsSNIHost
	}
	return gravityConfig, nil
}

// ProcessConfig defines the configuration for the wizard process
type ProcessConfig struct {
	// ProcessConfig specifies base process configuration
	ossinstall.ProcessConfig
	// OpsAdvertiseAddr is the Ops Center advertise address
	OpsAdvertiseAddr string
	// OpsSNIHost is the Ops Center SNI host
	OpsSNIHost string
}
