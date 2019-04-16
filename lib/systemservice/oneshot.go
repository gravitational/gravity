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

// ReinstallOneshotServiceSimple installs a systemd service named serviceName of type=oneshot
// using args as arguments to the gravity binary.
// The service will use the same binary as running this process.
// The service will also be configured to appear running after exit
// (https://www.freedesktop.org/software/systemd/man/systemd.service.html#RemainAfterExit=).
// The operation is non-blocking and returns without waiting for service to start
func ReinstallOneshotServiceSimple(serviceName string, args ...string) error {
	services, err := New()
	if err != nil {
		return trace.Wrap(err)
	}
	args = append([]string{utils.Exe.Path}, args...)
	req := NewServiceRequest{
		ServiceSpec: ServiceSpec{
			// Output the gravity binary version as a start command
			StartCommand: fmt.Sprintf("%v version", utils.Exe.Path),
			// We do the actual job as a command executed before the service entrypoint
			// to distinguish between completed job (status active) and in-progress job
			// (status activating)
			StartPreCommand: strings.Join(args, " "),
			User:            constants.RootUIDString,
			RemainAfterExit: true,
		},
		NoBlock: true,
		Name:    serviceName,
	}
	return trace.Wrap(installOneshotService(services, req))
}

// ReinstallOneshotService installs a systemd service specified with req.
// The operation is non-blocking and returns without waiting for service to start
func ReinstallOneshotService(req NewServiceRequest) error {
	services, err := New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(installOneshotService(services, req))
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

func installOneshotService(services ServiceManager, req NewServiceRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	req.ServiceSpec.Type = constants.OneshotService
	if req.ServiceSpec.User == "" {
		req.ServiceSpec.User = constants.RootUIDString
	}
	err := services.StopService(req.Name)
	if err != nil && !isUnknownServiceError(err) {
		log.WithField("service", req.Name).Warn("Failed to stop.")
	}
	return trace.Wrap(services.InstallService(req))
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
