package systemservice

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// InstallOneshotService installs a systemd service named serviceName of type=oneshot
// using args as arguments to the gravity binary.
// The service will use the same binary running this process.
// The operation is non-blocking and returns without waiting for service to start
func InstallOneshotService(serviceName string, args ...string) error {
	services, err := New()
	if err != nil {
		return trace.Wrap(err)
	}
	args = append([]string{utils.Exe.Path}, args...)
	spec := ServiceSpec{
		// Output the gravity binary version as a start command
		StartCommand: fmt.Sprintf("%v version", gravityPath),
		// We do actual job as a command executed before the service entrypoint
		// to distinguish between completed job (status active) and in-progress job
		// (status activating)
		StartPreCommand: strings.Join(args, " "),
		User:            "planet",
	}
	return trace.Wrap(installOneshotServiceFromSpec(service, serviceName, spec, args...))
}

// InstallOneshotServiceFromSpec installs a systemd service named serviceName of type=oneshot
// using args as the service command.
// The operation is non-blocking and returns without waiting for service to start
func InstallOneshotServiceFromSpec(serviceName string, spec ServiceSpec, args ...string) error {
	services, err := New()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(installOneshotServiceFromSpec(service, serviceName, spec, args...))
}

func installOneshotServiceFromSpec(service ServiceManager, serviceName string, spec ServiceSpec, args ...string) error {
	spec.Type = constants.OneshotService
	spec.RemainAfterExit = true
	if spec.User == "" {
		spec.User = constants.RootUIDString
	}
	err := services.InstallService(NewServiceRequest{
		Name:        serviceName,
		NoBlock:     true,
		ServiceSpec: spec,
	})
	return trace.Wrap(err)
}
