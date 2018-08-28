package cli

import (
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/network"

	"github.com/gravitational/trace"
)

func enablePromiscMode(env *localenv.LocalEnvironment, ifaceName string) error {
	return trace.Wrap(network.SetPromiscuousMode(ifaceName))
}

func disablePromiscMode(env *localenv.LocalEnvironment, ifaceName string) error {
	return trace.Wrap(network.UnsetPromiscuousMode(ifaceName))
}
