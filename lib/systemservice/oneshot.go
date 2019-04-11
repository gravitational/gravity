package systemservice

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ReinstallOneshotService installs a systemd service named serviceName of type=oneshot
// using args as arguments to the gravity binary.
// The service will use the same binary running this process.
// The operation is non-blocking and returns without waiting for service to start
func ReinstallOneshotService(serviceName string, args ...string) error {
	services, err := New()
	if err != nil {
		return trace.Wrap(err)
	}
	args = append([]string{utils.Exe.Path}, args...)
	spec := ServiceSpec{
		// Output the gravity binary version as a start command
		StartCommand: fmt.Sprintf("%v version", utils.Exe.Path),
		// We do the actual job as a command executed before the service entrypoint
		// to distinguish between completed job (status active) and in-progress job
		// (status activating)
		StartPreCommand: strings.Join(args, " "),
		User:            constants.RootUIDString,
	}
	return trace.Wrap(installOneshotServiceFromSpec(services, serviceName, spec, args...))
}

// ReinstallOneshotServiceFromSpec installs a systemd service named serviceName of type=oneshot
// using args as the service command.
// The operation is non-blocking and returns without waiting for service to start
func ReinstallOneshotServiceFromSpec(serviceName string, spec ServiceSpec, args ...string) error {
	services, err := New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(installOneshotServiceFromSpec(services, serviceName, spec, args...))
}

// UninstallService uninstalls service with the specified name
func UninstallService(serviceName string) error {
	services, err := New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(services.UninstallService(serviceName))
}

// StartOneshotService starts service with the specified name if it's not already running.
// The service is started in non-blocking mode
func StartOneshotService(serviceName string) error {
	services, err := New()
	if err != nil {
		return trace.Wrap(err)
	}
	noBlock := true
	return trace.Wrap(services.StartService(serviceName, noBlock))
}

func installOneshotServiceFromSpec(services ServiceManager, serviceName string, spec ServiceSpec, args ...string) error {
	spec.Type = constants.OneshotService
	spec.RemainAfterExit = true
	if spec.User == "" {
		spec.User = constants.RootUIDString
	}

	err := services.StopService(serviceName)
	if err != nil && !isUnknownServiceError(err) {
		log.WithField("service", serviceName).Warn("Failed to stop.")
	}

	err = services.InstallService(NewServiceRequest{
		Name:        serviceName,
		NoBlock:     true,
		ServiceSpec: spec,
	})
	return trace.Wrap(err)
}

func isUnknownServiceError(err error) bool {
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
