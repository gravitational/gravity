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

package phases

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/devicemapper"
	"github.com/gravitational/gravity/lib/fsm"
	statedir "github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/mount"
	"github.com/gravitational/gravity/lib/system/state"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// DockerDevicemapper is a phase executor that deals with Docker devicemapper devices.
type DockerDevicemapper struct {
	logrus.FieldLogger
}

// NewDockerDevicemapper returns phase executor that deals with Docker devicemapper devices.
func NewDockerDevicemapper(p fsm.ExecutorParams, log logrus.FieldLogger) (*DockerDevicemapper, error) {
	return &DockerDevicemapper{
		FieldLogger: log,
	}, nil
}

// Execute unmounts and removes Docker devicemapper devices.
func (d *DockerDevicemapper) Execute(ctx context.Context) error {
	d.Info("Removing devicemapper configuration.")
	err := devicemapper.Unmount(os.Stderr, d.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback configures Docker devicemapper devices back.
func (d *DockerDevicemapper) Rollback(ctx context.Context) error {
	return trace.NotImplemented("not implemented")
}

// DockerFormat is a phase executor that deals with formatting Docker devices.
type DockerFormat struct {
	logrus.FieldLogger
	Device string
}

// NewDockerFormat returns phase executor that deals with formatting Docker devices.
func NewDockerFormat(p fsm.ExecutorParams, log logrus.FieldLogger) (*DockerFormat, error) {
	node := *p.Phase.Data.Server
	return &DockerFormat{
		FieldLogger: log,
		Device:      node.Docker.Device.Path(),
	}, nil
}

// Execute creates filesystem on a Docker data device.
func (d *DockerFormat) Execute(ctx context.Context) error {
	d.Info("Formatting device %v.", d.Device)
	_, err := state.FormatDevice(d.Device)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback removes filesystem from the Docker data device.
func (d *DockerFormat) Rollback(ctx context.Context) error {
	return trace.NotImplemented("not implemented")
}

// DockerMount is a phase executor that deals with Docker data directory mounts.
type DockerMount struct {
	logrus.FieldLogger
	Device string
}

// NewDockerMount returns phase executor that deals with Docker data directory mounts.
func NewDockerMount(p fsm.ExecutorParams, log logrus.FieldLogger) (*DockerFormat, error) {
	node := *p.Phase.Data.Server
	return &DockerFormat{
		FieldLogger: log,
		Device:      node.Docker.Device.Path(),
	}, nil
}

// Execute creates a systemd mount for Docker data directory.
func (d *DockerMount) Execute(ctx context.Context) error {
	stateDir, err := statedir.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	config := mount.ServiceConfig{
		What:       storage.DeviceName(d.Device),
		Where:      filepath.Join(stateDir, defaults.PlanetDir, defaults.DockerDir),
		Filesystem: "xfs",
		Options:    []string{"defaults"},
	}
	d.Infof("Mounting %v to %v.", config.What, config.Where)
	err = mount.Mount(config)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback removes the systemd mount for Docker data directory.
func (d *DockerMount) Rollback(ctx context.Context) error {
	return trace.NotImplemented("not implemented")
}
