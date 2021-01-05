package install

import (
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// MakeProcessConfig creates a gravity process config from installer config
func MakeProcessConfig(i Config) (*processconfig.Config, error) {
	// first make the open-source config
	gravityConfig, err := install.MakeProcessConfig(i.Config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// now extend it with enterprise-specific values
	if i.OpsAdvertiseAddr != "" {
		// in case of Ops Center install, its SNI host is the advertise hostname
		gravityConfig.OpsCenter.SeedConfig.SNIHost, _ = utils.SplitHostPort(i.OpsAdvertiseAddr, "")
	} else {
		// in case of regular cluster install, the Ops Center SNI host might
		// have been provided on the CLI (e.g. by install instructions
		// generated by the Ops Center)
		gravityConfig.OpsCenter.SeedConfig.SNIHost = i.OpsSNIHost
	}
	return gravityConfig, nil
}
