package cli

import (
	"io/ioutil"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/install"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/e/lib/process"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/gravity/cli"

	"github.com/gravitational/trace"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/apimachinery/pkg/runtime"
)

// InstallConfig extends the open-source installer CLI config
type InstallConfig struct {
	// InstallConfig is the open-source installer CLI config
	cli.InstallConfig
	// License is the cluster License
	License string
	// OpsAdvertiseAddr is the advertise address configuration for Ops Center
	OpsAdvertiseAddr string
	// OpsURL is the URL of the remote Ops Center
	OpsURL string
	// OpsToken is the auth token of the remote Ops Center
	OpsToken string
	// OperationID is the existing operation ID when installing via Ops Center
	OperationID string
	// OpsTunnelToken is the token used to connect to remote Ops Center
	OpsTunnelToken string
	// OpsSNIHost is the remote Ops Center SNI host
	OpsSNIHost string
}

// NewInstallConfig creates install config from the passed CLI args and flags
func NewInstallConfig(g *Application) InstallConfig {
	return InstallConfig{
		InstallConfig:    cli.NewInstallConfig(g.Application),
		License:          *g.InstallCmd.License,
		OpsAdvertiseAddr: *g.InstallCmd.OpsAdvertiseAddr,
		OpsURL:           *g.InstallCmd.OpsCenterURL,
		OpsToken:         *g.InstallCmd.OpsCenterToken,
		OperationID:      *g.InstallCmd.OperationID,
		OpsTunnelToken:   *g.InstallCmd.OpsCenterTunnelToken,
		OpsSNIHost:       *g.InstallCmd.OpsCenterSNIHost,
	}
}

// CheckAndSetDefaults validates the configuration object and populates default values
func (i *InstallConfig) CheckAndSetDefaults() (err error) {
	if i.OpsAdvertiseAddr != "" {
		if _, _, err := utils.ParseHostPort(i.OpsAdvertiseAddr); err != nil {
			return trace.Wrap(err, "failed to parse Ops Center advertise "+
				"address %q specified with --ops-advertise-addr flag, make "+
				"sure it's in the <hostname>:<port> format", i.OpsAdvertiseAddr)
		}
	}
	if i.NewProcess == nil {
		i.NewProcess = process.NewProcess
	}
	if err := i.InstallConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ToInstallerConfig converts CLI config to installer format
func (i *InstallConfig) ToInstallerConfig(env *environment.Local) (*install.Config, error) {
	ossConfig, err := i.InstallConfig.ToInstallerConfig(env.LocalEnvironment, resources.ValidateFunc(gravity.Validate))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ossConfig.NewProcess = i.NewProcess
	var opsResources []runtime.Object
	if i.OpsAdvertiseAddr != "" {
		opsResources, err = ops.NewOpsCenterConfig(
			ops.OpsCenterConfigParams{
				AdvertiseAddr: i.OpsAdvertiseAddr,
				Devmode:       i.Insecure,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	ossConfig.RuntimeResources = append(ossConfig.RuntimeResources, opsResources...)
	return &install.Config{
		Config:           *ossConfig,
		OpsAdvertiseAddr: i.OpsAdvertiseAddr,
		License:          i.License,
		RemoteOpsURL:     i.OpsURL,
		RemoteOpsToken:   i.OpsToken,
		OperationID:      i.OperationID,
		OpsTunnelToken:   i.OpsTunnelToken,
		OpsSNIHost:       i.OpsSNIHost,
	}, nil
}

func parseArgs(args []string) (*kingpin.ParseContext, error) {
	app := kingpin.New("gravity", "")
	app.Terminate(func(int) {})
	app.Writer(ioutil.Discard)
	return RegisterCommands(app).ParseContext(args)
}
