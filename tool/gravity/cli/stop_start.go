/*
Copyright 2020 Gravitational, Inc.

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
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/systemservice"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
)

const systemStopMessage = `This action will stop all Gravity and Kubernetes services on the node.
Would you like to proceed? You can launch the command with --confirm flag to suppress this prompt in future.`

func stopGravity(env *localenv.LocalEnvironment, confirmed bool) error {
	if !confirmed {
		env.Println(color.YellowString(systemStopMessage))
		confirmed, err := confirm()
		if err != nil {
			return trace.Wrap(err)
		}
		if !confirmed {
			env.Println("Action cancelled by user.")
			return nil
		}
	}

	packages, err := findInstalledPackages(env.Packages)
	if err != nil {
		return trace.Wrap(err)
	}

	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, pack := range packages {
		env.PrintStep("Stopping system service %s", pack)
		if err := svm.StopPackageService(pack); err != nil {
			return trace.Wrap(err)
		}
		env.PrintStep("Disabling system service %s", pack)
		if err := svm.DisablePackageService(pack); err != nil {
			return trace.Wrap(err)
		}
	}

	env.PrintStep("Gravity services have been stopped and disabled")
	return nil
}

const systemStartMessage = `This action will start all Gravity and Kubernetes services on the node.
Would you like to proceed? You can launch the command with --confirm flag to suppress this prompt in future.`

func startGravity(env *localenv.LocalEnvironment, confirmed bool) error {
	if !confirmed {
		env.Println(color.YellowString(systemStartMessage))
		confirmed, err := confirm()
		if err != nil {
			return trace.Wrap(err)
		}
		if !confirmed {
			env.Println("Action cancelled by user.")
			return nil
		}
	}

	nodeAddr, err := env.Backend.GetNodeAddr()
	if err != nil {
		return trace.Wrap(err)
	}

	err = checkAdvertiseAddress(nodeAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	packages, err := findInstalledPackages(env.Packages)
	if err != nil {
		return trace.Wrap(err)
	}

	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, pack := range packages {
		env.PrintStep("Enabling system service %s", pack)
		if err := svm.EnablePackageService(pack); err != nil {
			return trace.Wrap(err)
		}
		env.PrintStep("Starting system service %s", pack)
		if err := svm.StartPackageService(pack, false); err != nil {
			return trace.Wrap(err)
		}
	}

	env.PrintStep("Gravity services have been enabled and started")
	return nil
}

func findInstalledPackages(packages pack.PackageService) ([]loc.Locator, error) {
	planet, err := pack.FindRuntimePackage(packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	teleport, err := pack.FindInstalledPackage(packages, loc.Teleport)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []loc.Locator{*teleport, *planet}, nil
}

// checkAdvertiseAddress makes sure that the node has a network interface for
// its configured advertise address.
//
// This helps prevent scenarios when, for example, a node is migrated to
// another machine and user does not provide a new advertise address to the
// start command.
func checkAdvertiseAddress(advertiseIP string) error {
	ifaces, err := systeminfo.NetworkInterfaces()
	if err != nil {
		return trace.Wrap(err)
	}
	var ips []string
	for _, iface := range ifaces {
		ips = append(ips, iface.IPv4)
	}
	for _, ip := range ips {
		if ip == advertiseIP {
			return nil
		}
	}
	return trace.NotFound(`The cluster node is configured with advertise address %v but it's not present on the machine.
Available addresses are: %v.
If you wish to reconfigure the node to use a different advertise address, use "gravity start --advertise-addr=<new-ip>" command.`, advertiseIP, ips)
}
