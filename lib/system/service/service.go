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
	"os"
	"path/filepath"

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

// IsFailed determines if the specified service has failed.
// Returns nil error if the service has failed and an error otherwise
func IsFailed(serviceName string) error {
	return IsStatus(serviceName, systemservice.ServiceStatusFailed)
}

// IsStatus checks whether the specified service has the given active status
func IsStatus(serviceName string, statuses ...string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	activeStatus, err := services.StatusService(serviceName)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, status := range statuses {
		if activeStatus == status {
			return nil
		}
	}
	return trace.CompareFailed("unexpected status %v", activeStatus)
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

// Name returns the unit name part of path
func Name(path string) string {
	return filepath.Base(path)
}

func install(services systemservice.ServiceManager, req systemservice.NewServiceRequest) error {
	if req.ServiceSpec.User == "" {
		req.ServiceSpec.User = constants.RootUIDString
	}
	logger := log.WithField("service", req.Name)
	err := services.DisableService(systemservice.DisableServiceRequest{
		Name: Name(req.Name),
		Now:  true,
	})
	if err != nil && !systemservice.IsUnknownServiceError(err) {
		logger.WithError(err).Warn("Failed to disable.")
	}
	return trace.Wrap(services.InstallService(req))
}

func removeLingeringUnitFile(servicePath string) error {
	defaultPath := systemservice.DefaultUnitPath(Name(servicePath))
	if defaultPath == servicePath {
		return nil
	}
	if err := os.Remove(defaultPath); err != nil && !os.IsNotExist(err) {
		return trace.ConvertSystemError(err)
	}
	log.WithField("unit-file", defaultPath).Info("Removed lingering unit file.")
	return nil
}
