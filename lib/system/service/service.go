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

// package service implements helpers for working with systemd services
package service

import (
	"os/exec"
	"syscall"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/systemservice"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Uninstall uninstalls service with the specified name
func Uninstall(req systemservice.UninstallServiceRequest) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(services.UninstallService(req))
}

// Disable disables service with the specified name
func Disable(req systemservice.DisableServiceRequest) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(services.DisableService(req))
}

// Start starts service with the specified name if it's not already running.
// The service is started in non-blocking mode
func Start(serviceName string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	noBlock := true
	return trace.Wrap(services.StartService(serviceName, noBlock))
}

// Reinstall installs a systemd service specified with req.
// The operation is non-blocking and returns without waiting for service to start
func Reinstall(req systemservice.NewServiceRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(install(services, req))
}

func install(services systemservice.ServiceManager, req systemservice.NewServiceRequest) error {
	if req.ServiceSpec.User == "" {
		req.ServiceSpec.User = constants.RootUIDString
	}
	err := services.StopService(req.Name)
	if err != nil && !IsUnknownServiceError(err) {
		log.WithField("service", req.Name).Warn("Failed to stop.")
	}
	return trace.Wrap(services.InstallService(req))
}

// IsUnknownServiceError determines whether the err specifies the
// 'unknown service' error
func IsUnknownServiceError(err error) bool {
	const (
		errCodeGenericFailure = 1
		errCodeNotInstalled   = 5
	)
	switch err := trace.Unwrap(err).(type) {
	case *exec.ExitError:
		if status, ok := err.Sys().(syscall.WaitStatus); ok {
			switch status.ExitStatus() {
			case errCodeGenericFailure, errCodeNotInstalled:
				return true
			}
		}
	}
	return false
}
