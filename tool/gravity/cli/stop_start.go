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
	"github.com/gravitational/gravity/lib/systemservice"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
)

const systemStopMessage = `This action will stop all Gravity and Kubernetes services on the node.
Would you like to proceed? You can launch the command with --confirm flag to suppress this prompt in future.`

func stopGravity(env *localenv.LocalEnvironment, confirmed, disable bool) error {
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
		err := svm.StopPackageService(pack)
		if err != nil {
			return trace.Wrap(err)
		}
		if disable {
			env.PrintStep("Disabling system service %s", pack)
			err := svm.DisablePackageService(pack)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	if disable {
		env.PrintStep("Gravity services have been stopped and disabled")
	} else {
		env.PrintStep("Gravity services have been stopped")
	}

	return nil
}

const systemStartMessage = `This action will start all Gravity and Kubernetes services on the node.
Would you like to proceed? You can launch the command with --confirm flag to suppress this prompt in future.`

func startGravity(env *localenv.LocalEnvironment, confirmed, enable bool) error {
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

	packages, err := findInstalledPackages(env.Packages)
	if err != nil {
		return trace.Wrap(err)
	}

	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, pack := range packages {
		if enable {
			env.PrintStep("Enabling system service %s", pack)
			err := svm.EnablePackageService(pack)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		env.PrintStep("Starting system service %s", pack)
		err := svm.StartPackageService(pack, false)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if enable {
		env.PrintStep("Gravity services have been enabled and started")
	} else {
		env.PrintStep("Gravity services have been started")
	}

	return nil
}

func findInstalledPackages(packages pack.PackageService) ([]loc.Locator, error) {
	planet, err := pack.FindInstalledPackage(packages, loc.Planet)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	teleport, err := pack.FindInstalledPackage(packages, loc.Teleport)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []loc.Locator{*teleport, *planet}, nil
}
