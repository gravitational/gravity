package cli

import (
	"context"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/schema"
	libstatus "github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

// planetEnter is a shortcut that finds installed planet in this cluster
// and enters it
func planetEnter(env *localenv.LocalEnvironment, args []string) error {
	planetPackage, planetConfigPackage, err := findAnyRuntimePackageWithConfig(env.Packages)
	if err != nil {
		return trace.Wrap(err)
	}
	return executePackageCommand(
		env, "enter", *planetPackage, planetConfigPackage, args)
}

// planetShell is a shortcut that finds installed planet in this cluster
// and enters it
func planetShell(env *localenv.LocalEnvironment) error {
	return planetExec(env, true, true, "/bin/bash", nil)
}

// planetExec executes a command within a namespace of a planet container
func planetExec(env *localenv.LocalEnvironment, tty bool, stdin bool, cmd string, extraArgs []string) error {
	planetPackage, planetConfigPackage, err := findAnyRuntimePackageWithConfig(env.Packages)
	if err != nil {
		return trace.Wrap(err)
	}
	var args []string
	if tty {
		args = append(args, "-t")
	}
	if stdin {
		args = append(args, "-i")
	}
	args = append(args, cmd)
	args = append(args, extraArgs...)
	return executePackageCommand(
		env, "exec", *planetPackage, planetConfigPackage, args)
}

func getPlanetStatus(env *localenv.LocalEnvironment, args []string) error {
	planetPackage, planetConfigPackage, err := findAnyRuntimePackageWithConfig(env.Packages)
	if err != nil {
		return trace.Wrap(err)
	}

	certFile, err := localenv.InGravity(defaults.SecretsDir, "root.cert")
	if err != nil {
		return trace.Wrap(err)
	}

	args = append(args, "--cert-file", certFile)
	return executePackageCommand(
		env, "status", *planetPackage, planetConfigPackage, args)
}

// planetVersion returns version of the currently installed planet
func planetVersion(env *localenv.LocalEnvironment) (*semver.Version, error) {
	locator, err := findAnyRuntimePackage(env.Packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	version, err := locator.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return version, nil
}

// getMasterNodes returns IPs of cluster machines running planet masters
//
// This method is supposed to be called from inside the planet container.
func getMasterNodes(ctx context.Context, servers []storage.Server) (addrs []string, err error) {
	status, err := libstatus.FromPlanetAgent(ctx, servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, node := range status.Nodes {
		if node.Role == string(schema.ServiceRoleMaster) {
			addrs = append(addrs, node.AdvertiseIP)
		}
	}
	return addrs, nil
}
