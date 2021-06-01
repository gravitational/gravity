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

package cli

import (
	"context"
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	libstatus "github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	agentpb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/gravitational/trace"
)

// planetEnter is a shortcut that finds installed planet in this cluster
// and enters it
func planetEnter(env *localenv.LocalEnvironment, args []string) error {
	planetPackage, planetConfigPackage, err := pack.FindAnyRuntimePackageWithConfig(env.Packages)
	if err != nil {
		return trace.Wrap(err)
	}
	var cmd []string
	if env.SELinux {
		cmd = append(cmd, "--selinux")
	}
	if env.Debug {
		cmd = append(cmd, "--debug")
	}
	cmd = append(cmd, args...)
	return executePackageCommand(
		env, "enter", *planetPackage, planetConfigPackage, cmd)
}

// planetShell is a shortcut that finds installed planet in this cluster
// and enters it
func planetShell(env *localenv.LocalEnvironment) error {
	return planetExec(env, true, true, "/bin/bash", nil)
}

// planetExec executes a command within a namespace of a planet container
func planetExec(env *localenv.LocalEnvironment, tty bool, stdin bool, cmd string, extraArgs []string) error {
	planetPackage, planetConfigPackage, err := pack.FindAnyRuntimePackageWithConfig(env.Packages)
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
	if env.SELinux {
		args = append(args, "--selinux")
	}
	if env.Debug {
		args = append(args, "--debug")
	}
	args = append(args, cmd)
	args = append(args, extraArgs...)
	return executePackageCommand(
		env, "exec", *planetPackage, planetConfigPackage, args)
}

func getPlanetStatus(env *localenv.LocalEnvironment, args []string) error {
	planetPackage, planetConfigPackage, err := pack.FindAnyRuntimePackageWithConfig(env.Packages)
	if err != nil {
		return trace.Wrap(err)
	}

	caFile, err := localenv.InGravity(defaults.SecretsDir, defaults.RootCertFilename)
	if err != nil {
		return trace.Wrap(err)
	}
	clientCertFile, err := localenv.InGravity(defaults.SecretsDir, fmt.Sprint(constants.PlanetRPCKeyPair, ".", utils.CertSuffix))
	if err != nil {
		return trace.Wrap(err)
	}
	clientKeyFile, err := localenv.InGravity(defaults.SecretsDir, fmt.Sprint(constants.PlanetRPCKeyPair, ".", utils.KeySuffix))
	if err != nil {
		return trace.Wrap(err)
	}

	args = append(args, "--ca-file", caFile)
	args = append(args, "--client-cert-file", clientCertFile)
	args = append(args, "--client-key-file", clientKeyFile)
	return executePackageCommand(
		env, "status", *planetPackage, planetConfigPackage, args)
}

// getMasterNodes returns IPs of cluster machines running planet masters
//
// This method is supposed to be called from inside the planet container.
func getMasterNodes(ctx context.Context, servers []storage.Server) (addrs []string, err error) {
	status, err := libstatus.FromPlanetAgent(ctx, servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if status.GetSystemStatus() != agentpb.SystemStatus_Running {
		return nil, trace.BadParameter("cluster is degraded")
	}

	for _, node := range status.Nodes {
		if node.Role == string(schema.ServiceRoleMaster) {
			addrs = append(addrs, node.AdvertiseIP)
		}
	}
	return addrs, nil
}
