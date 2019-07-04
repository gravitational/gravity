/*
Copyright 2018 Gravitational, Inc.

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

package mount

import (
	"strings"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systemservice"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Mount creates and starts a new systemd mount.
func Mount(config ServiceConfig) error {
	serviceManager, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	err = MountService(config, config.ServiceName(), serviceManager)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Unmount uninstalls specified systemd mount service.
func Unmount(serviceName string) error {
	serviceManager, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	err = UnmountService(serviceName, serviceManager)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// MountService creates a new mount based on the given configuration.
// The mount is created as a systemd mount unit named service.
func MountService(config ServiceConfig, service string, services systemservice.ServiceManager) error {
	spec := systemservice.MountServiceSpec{
		Where: config.Where,
		What:  storage.DeviceName(config.What).Path(),
		Type:  config.Filesystem,
	}
	req := systemservice.NewMountServiceRequest{
		ServiceSpec: spec,
		Name:        service,
	}

	err := services.StopService(service)
	if err != nil {
		log.Warnf("Error stopping service %v: %v.", service, trace.DebugReport(err))
	}

	err = services.InstallMountService(req)
	if err != nil {
		return trace.Wrap(err, "failed to install mount service %q", service)
	}
	return nil
}

// UnmountService uninstalls the specified mount service.
func UnmountService(service string, services systemservice.ServiceManager) error {
	status, err := services.StatusService(service)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Debugf("Mount service is %q.", status)
	err = services.UninstallService(service)
	if err != nil {
		return trace.Wrap(err, "failed to uninstall mount service %q", service)
	}

	return nil
}

// ServiceConfig describes configuration to mount a directory
// on a specific device and filesystem
//
// See https://www.freedesktop.org/software/systemd/man/systemd.mount.html
type ServiceConfig struct {
	// What specifies defines the absolute path of a device node, file or other resource to mount
	What storage.DeviceName
	// Where specifies the absolute path of a directory for the mount point
	Where string
	// Filesystem specifies the file system type
	Filesystem string
	// Options lists mount options to use when mounting
	Options []string
}

// ServiceName returns name of the mount service.
func (c ServiceConfig) ServiceName() string {
	return strings.Replace(c.Where, "/", "-", -1)[1:] + ".mount"
}
