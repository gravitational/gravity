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

package service

import (
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/systemservice"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// ReinstallSimpleService reinstalls the cmd as a simple service with the
// specified serviceName.
func ReinstallSimpleService(serviceName string, cmd []string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	err = services.StopService(serviceName)
	if err != nil {
		logrus.WithError(err).Warnf("Error stopping service %v.", serviceName)
	}

	err = services.InstallService(systemservice.NewServiceRequest{
		Name:                serviceName,
		NoBlock:             true,
		ReloadConfiguration: true,
		ServiceSpec: systemservice.ServiceSpec{
			User:         constants.RootUIDString,
			Type:         SimpleService,
			StartCommand: strings.Join(cmd, " "),
			Restart:      RestartOnFailure,
			WantedBy:     defaults.SystemServiceWantedBy,
		},
	})
	return trace.Wrap(err)
}
