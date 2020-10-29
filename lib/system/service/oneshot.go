/*
Copyright 2019 Gravitational, Inc.

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

package service

import (
	"fmt"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// ReinstallOneshotSimple installs a systemd service named serviceName of type=oneshot
// using args as arguments to the gravity binary.
// The service will use the same binary as running this process.
// The service will also be configured to appear running after exit
// (https://www.freedesktop.org/software/systemd/man/systemd.service.html#RemainAfterExit=).
// The operation is non-blocking and returns without waiting for service to start
func ReinstallOneshotSimple(serviceName string, args ...string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	args = append([]string{utils.Exe.Path}, args...)
	req := systemservice.NewServiceRequest{
		ServiceSpec: systemservice.ServiceSpec{
			// Output the gravity binary version as a start command
			StartCommand: fmt.Sprintf("%v version", utils.Exe.Path),
			// We do the actual job as a command executed before the service entrypoint
			// to distinguish between completed job (status active) and in-progress job
			// (status activating)
			StartPreCommands: []string{strings.Join(args, " ")},
			User:             constants.RootUIDString,
			RemainAfterExit:  true,
		},
		NoBlock: true,
		Name:    serviceName,
	}
	return trace.Wrap(installOneshot(services, req))
}

// ReinstallOneshot installs a systemd service specified with req.
// The operation is non-blocking and returns without waiting for service to start
func ReinstallOneshot(req systemservice.NewServiceRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(installOneshot(services, req))
}

func installOneshot(services systemservice.ServiceManager, req systemservice.NewServiceRequest) error {
	req.ServiceSpec.Type = OneshotService
	return trace.Wrap(reinstall(services, req))
}
