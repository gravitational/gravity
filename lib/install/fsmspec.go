package install

import (
	"strings"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install/phases"

	"github.com/gravitational/trace"
)

// FSMSpec returns a function that returns an appropriate phase executor
// based on the provided params
func FSMSpec(config FSMConfig) fsm.FSMSpecFunc {
	return func(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
		switch {
		case p.Phase.ID == phases.ChecksPhase:
			return phases.NewChecks(p,
				config.Operator,
				config.OperationKey)

		case p.Phase.ID == phases.ConfigurePhase:
			return phases.NewConfigure(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, phases.BootstrapPhase):
			return phases.NewBootstrap(p,
				config.Operator,
				config.Apps,
				config.LocalBackend, remote)

		case strings.HasPrefix(p.Phase.ID, phases.PullPhase):
			return phases.NewPull(p,
				config.Operator,
				config.Packages,
				config.LocalPackages,
				config.Apps,
				config.LocalApps, remote)

		case strings.HasPrefix(p.Phase.ID, phases.MastersPhase), strings.HasPrefix(p.Phase.ID, phases.NodesPhase):
			return phases.NewSystem(p,
				config.Operator, remote)

		case p.Phase.ID == phases.WaitPhase:
			return phases.NewWait(p,
				config.Operator)

		case p.Phase.ID == phases.LabelPhase:
			return phases.NewNodes(p,
				config.Operator,
				config.LocalApps)

		case p.Phase.ID == phases.RBACPhase:
			return phases.NewRBAC(p,
				config.Operator,
				config.LocalApps)

		case p.Phase.ID == phases.ResourcesPhase:
			return phases.NewResources(p,
				config.Operator)

		case strings.HasPrefix(p.Phase.ID, phases.ExportPhase):
			return phases.NewExport(p,
				config.Operator,
				config.LocalPackages,
				config.LocalApps, remote)

		case strings.HasPrefix(p.Phase.ID, phases.RuntimePhase), strings.HasPrefix(p.Phase.ID, phases.AppPhase):
			return phases.NewApp(p,
				config.Operator,
				config.LocalApps)

		case strings.HasPrefix(p.Phase.ID, phases.EnableElectionPhase):
			return phases.NewEnableElectionPhase(p, config.Operator)

		default:
			return nil, trace.BadParameter("unknown phase %q", p.Phase.ID)
		}
	}
}
