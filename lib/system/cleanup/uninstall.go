package cleanup

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
	if err := uninstallPackageServices(printer, logger); err != nil {
		errors = append(errors, err)
	}
	if err := unmountDevicemapper(printer, logger); err != nil {
		errors = append(errors, err)
	}
	if err := removeInterfaces(printer); err != nil {
		errors = append(errors, err)
	}
	if err := removeStateDirectories(printer, logger); err != nil {
		errors = append(errors, err)
	}
	for _, targetPath := range state.GravityBinPaths {
		err = os.Remove(targetPath)
		if err == nil {
			printer.PrintStep("Removed gravity binary %v", targetPath)
			break
		}
	}
	if err != nil {
		log.WithError(err).Warn("Failed to delete gravity binary.")
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// UninstallAgentServices stops and uninstalls agent services (installer agent and/or service)
func UninstallAgentServices(logger log.FieldLogger) error {
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	var errors []error
	for _, service := range []string{
		defaults.GravityRPCAgentServiceName,
		defaults.GravityRPCInstallerServiceName,
	} {
		if err := svm.UninstallService(service); err != nil {
			logger.WithError(err).Warn("Failed to uninstall agent service.")
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
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
		if err := svm.DisableService(service); err != nil {
			logger.WithError(err).Warn("Failed to disable agent service.")
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func uninstallPackageServices(printer utils.Printer, logger log.FieldLogger) error {
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	services, err := svm.ListPackageServices()
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
		if err := svm.UninstallPackageService(service.Package); err != nil {
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

func removeStateDirectories(printer utils.Printer, logger log.FieldLogger) error {
	var errors []error
	if stateDir, err := state.GetStateDir(); err == nil {
		printer.PrintStep("Deleting all local data at %v", stateDir)
		if err = os.RemoveAll(stateDir); err != nil {
			// do not fail if the state directory cannot be removed, probably
			// this means it is a mount
			logger.WithError(err).Warn("Failed to remove %v.", stateDir)
		}
	} else {
		errors = append(errors, err)
	}
	// remove all files and directories gravity might have created on the system
	for _, path := range append(state.StateLocatorPaths,
		defaults.ModulesPath,
		defaults.SysctlPath,
		defaults.GravityEphemeralDir,
		defaults.GravityInstallDir(),
	) {
		// errors are expected since some of them may not exist
		err := os.RemoveAll(path)
		if err == nil {
			printer.PrintStep("Removed %v", path)
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
		if utils.HasOneOfPrefixes(iface.Name, "docker", "flannel", "cni") {
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
	command := exec.Command("gravity", "enter", "--", "--notty", "/usr/bin/docker", "--", "info")
	err := utils.Exec(command, &out)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query docker info: %s", out)
	}
	return utils.ParseDockerInfo(&out)
}
