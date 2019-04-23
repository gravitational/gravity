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
func Uninstall(serviceName string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(services.UninstallService(serviceName))
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
