package install

import (
	"strings"

	"github.com/gravitational/gravity/e/lib/install/phases"
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/e/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/install"
	libinstall "github.com/gravitational/gravity/lib/install/phases"
	ossgravity "github.com/gravitational/gravity/lib/ops/resources/gravity"

	"github.com/gravitational/trace"
)

func init() {
	// Override the default install FSM spec with the enterprise version
	// that supports additional enterprise-specific phases.
	install.FSMSpec = FSMSpec
}

// FSMSpec returns a function that returns an appropriate phase executor
// based on the provided params
func FSMSpec(config install.FSMConfig) fsm.FSMSpecFunc {
	return func(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
		switch {
		case p.Phase.ID == phases.InstallerPhase:
			return phases.NewInstaller(p,
				config.Operator,
				config.Packages,
				config.Apps)

		case p.Phase.ID == phases.DecryptPhase:
			return phases.NewDecrypt(p,
				config.Operator,
				config.Packages,
				config.Apps)

		case p.Phase.ID == phases.LicensePhase:
			client, _, err := httplib.GetClusterKubeClient(p.Plan.DNSConfig.Addr())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return phases.NewLicense(p,
				config.Operator,
				client)

		case p.Phase.ID == phases.ConnectPhase:
			return phases.NewConnect(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, phases.ClusterPhase):
			return phases.NewCluster(p,
				config.Operator,
				config.Packages,
				config.Apps,
				config.UserLogFile)

		case strings.HasPrefix(p.Phase.ID, libinstall.GravityResourcesPhase):
			ossOperator, err := config.LocalClusterClient()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			ossResources, err := ossgravity.New(ossgravity.Config{
				Operator: ossOperator,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			operator := client.New(ossOperator)
			factory, err := gravity.New(gravity.Config{
				Resources: ossResources,
				Operator:  operator,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return libinstall.NewGravityResourcesPhase(p, operator, factory)
		}

		// none of enterprise-specific phases matched, check open-source
		return install.DefaultFSMSpec(config)(p, remote)
	}
}
