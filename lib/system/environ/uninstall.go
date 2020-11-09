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

package environ

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/devicemapper"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// UninstallSystem removes all state from the system on best-effort basis
func UninstallSystem(printer utils.Printer, logger log.FieldLogger) (err error) {
	var errors []error
	if err := unmountDevicemapper(printer, logger); err != nil {
		errors = append(errors, err)
	}
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := uninstallPackageServices(svm, printer, logger); err != nil {
		errors = append(errors, err)
	}
	if err := removeInterfaces(printer); err != nil {
		errors = append(errors, err)
	}
	pathsToRemove := append(getStateDirectories(), state.GravityBinPaths...)
	pathsToRemove = append(pathsToRemove, state.KubectlBinPaths...)
	if err := removePaths(printer, logger, pathsToRemove...); err != nil {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// CleanupOperationState removes all operation state after the operation is complete
func CleanupOperationState(printer utils.Printer, logger log.FieldLogger) error {
	stateDir, err := state.GravityInstallDir()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(removePaths(printer, logger, stateDir))
}

// UninstallPackageServices stops and uninstalls system package services
func UninstallPackageServices(printer utils.Printer, logger log.FieldLogger) error {
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return uninstallPackageServices(svm, printer, logger)
}

// UninstallAgentServices stops and uninstalls system agent services
func UninstallAgentServices(printer utils.Printer, logger log.FieldLogger) error {
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return uninstallAgentServices(svm)
}

// UninstallServices stops and uninstalls all relevant services
func UninstallServices(printer utils.Printer, logger log.FieldLogger) error {
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	var errors []error
	if err := uninstallPackageServices(svm, printer, logger); err != nil {
		errors = append(errors, err)
	}
	if err := uninstallAgentServices(svm); err != nil {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// UninstallService stops and uninstalls a service with the specified name
func UninstallService(service string) error {
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(uninstallServices(svm, service))
}

// DisableAgentServices disables agent services (installer agent and/or service) without
// stopping
func DisableAgentServices(logger log.FieldLogger) error {
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	var errors []error
	for _, service := range []string{
		defaults.GravityRPCAgentServiceName,
		defaults.GravityRPCInstallerServiceName,
	} {
		req := systemservice.DisableServiceRequest{
			Name: service,
		}
		if err := svm.DisableService(req); err != nil && !systemservice.IsUnknownServiceError(err) {
			logger.WithError(err).Warn("Failed to disable agent service.")
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func uninstallAgentServices(svm systemservice.ServiceManager) error {
	return uninstallServices(svm,
		defaults.GravityRPCInstallerServiceName,
		defaults.GravityRPCAgentServiceName)
}

func uninstallPackageServices(svm systemservice.ServiceManager, printer utils.Printer, logger log.FieldLogger) error {
	services, err := svm.ListPackageServices(systemservice.DefaultListServiceOptions)
	if err != nil {
		return trace.Wrap(err)
	}
	var errors []error
	sort.Slice(services, func(i, j int) bool {
		// Move teleport package to the front of uninstall chain.
		// The reason for this is, if uninstalling the planet package would fail,
		// the node would continue sending heartbeats that would make it persist
		// in the list of nodes although it might have already been removed from
		// everywhere else during shrink.
		return services[i].Package.Name == constants.TeleportPackage
	})
	for _, service := range services {
		printer.PrintStep("Uninstalling system service %v", service)
		log := logger.WithField("package", service.Package)
		err := svm.UninstallPackageService(service.Package)
		if err != nil && systemservice.IsUnknownServiceError(err) {
			log.WithError(err).Warn("Failed to uninstall service.")
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func unmountDevicemapper(printer utils.Printer, logger log.FieldLogger) error {
	dockerInfo, err := dockerInfo()
	if err != nil {
		logger.WithError(err).Warn("Failed to get docker info.")
	} else {
		logger.WithField("info", fmt.Sprintf("%#v", dockerInfo)).Debug("Detected docker configuration.")
	}
	var out bytes.Buffer
	if dockerInfo != nil && dockerInfo.StorageDriver == constants.DockerStorageDriverDevicemapper {
		printer.PrintStep("Detected devicemapper, cleaning up disks")
		if err = devicemapper.Unmount(&out, logger); err != nil {
			log.WithFields(log.Fields{
				log.ErrorKey: err,
				"stdout":     out.String(),
			}).Warn("Failed to unmount devicemapper.")
			return trace.Wrap(err)
		}
	}
	return nil
}

func removePaths(printer utils.Printer, logger log.FieldLogger, paths ...string) error {
	var errors []error
	// remove all files and directories gravity might have created on the system
	for _, path := range paths {
		err := os.RemoveAll(path)
		if err == nil {
			printer.PrintStep("Removed %v", path)
			continue
		}
		if os.IsNotExist(err) || utils.IsResourceBusyError(err) {
			continue
		}
		logger.WithFields(log.Fields{
			log.ErrorKey: err,
			"path":       path,
		}).Warn("Failed to remove.")
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

func removeInterfaces(printer utils.Printer) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return trace.Wrap(err)
	}
	var errors []error
	for _, iface := range ifaces {
		if utils.HasOneOfPrefixes(iface.Name, "docker", "flannel", "cni", "wormhole") {
			printer.PrintStep("Removing network interface %q", iface.Name)
			var out bytes.Buffer
			if err := utils.Exec(exec.Command("ip", "link", "del", iface.Name), &out); err != nil {
				log.WithFields(log.Fields{
					log.ErrorKey: err,
					"interface":  iface.Name,
				}).Warn("Failed to remove interface.")
				errors = append(errors, err)
			}
		}
	}
	return trace.NewAggregate(errors...)
}

func dockerInfo() (*utils.DockerInfo, error) {
	var out bytes.Buffer
	args := utils.Exe.PlanetCommandArgs("/usr/bin/docker", "--", "info")
	command := exec.Command(args[0], args[1:]...)
	err := utils.Exec(command, &out)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query docker info: %s", out.String())
	}
	return utils.ParseDockerInfo(&out)
}

func getStateDirectories() (dirs []string) {
	stateDir, err := state.GetStateDir()
	if err == nil {
		dirs = append(dirs, stateDir)
	}
	dirs = append(dirs, state.StateLocatorPaths...)
	if stateDir, err := state.GravityInstallDir(); err == nil {
		dirs = append(dirs, stateDir)
	}
	return append(dirs,
		defaults.ModulesPath,
		defaults.SysctlPath,
		defaults.GravityEphemeralDir,
	)
}

func uninstallServices(svm systemservice.ServiceManager, services ...string) error {
	var errors []error
	for _, service := range services {
		req := systemservice.UninstallServiceRequest{
			Name: service,
		}
		if err := svm.UninstallService(req); err != nil && !systemservice.IsUnknownServiceError(err) {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}
