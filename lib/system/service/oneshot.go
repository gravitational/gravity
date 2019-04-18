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
	req.ServiceSpec.Type = constants.OneshotService
	return trace.Wrap(install(services, req))
}
