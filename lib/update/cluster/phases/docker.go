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
	"fmt"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/devicemapper"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system"
	"github.com/gravitational/gravity/lib/system/mount"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// dockerDevicemapper is a phase executor that handles migration of devicemapper
// devices to be used for overlay.
type dockerDevicemapper struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// Node is the node where devicemapper configuration should be updated.
	Node storage.Server
	// Device is the absolute path to the Docker device, e.g. /dev/vdb.
	Device string
	// Remote allows to invoke remote commands.
	Remote fsm.Remote
}

// NewDockerDevicemapper returns phase executor that handles migration of
// devicemapper devices to be used for overlay.
func NewDockerDevicemapper(p fsm.ExecutorParams, remote fsm.Remote, log logrus.FieldLogger) (*dockerDevicemapper, error) {
	node := *p.Phase.Data.Server
	return &dockerDevicemapper{
		FieldLogger: log,
		Node:        node,
		Device:      getDockerDevice(p.Phase.Data),
		Remote:      remote,
	}, nil
}

// Execute unmounts and removes Docker devicemapper devices.
func (d *dockerDevicemapper) Execute(ctx context.Context) error {
	d.Infof("Removing devicemapper configuration from %v.", d.Device)
	err := devicemapper.Unmount(os.Stderr, d.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}
	d.Infof("Devicemapper configuration on %v removed.", d.Device)
	return nil
}

// Rollback configures Docker devicemapper devices back.
func (d *dockerDevicemapper) Rollback(ctx context.Context) error {
	d.Infof("Restoring devicemapper configuration on %v.", d.Device)
	err := devicemapper.Mount(d.Device, os.Stderr, d.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}
	d.Infof("Devicemapper configuration on %v restored.", d.Device)
	// Since we recreated devicemapper environment from scratch,
	// wipe out Docker data directory to let it reinitialize,
	// otherwise it will fail to start with UUID mismatch.
	dockerDir, err := state.DockerDir()
	if err != nil {
		return trace.Wrap(err)
	}
	err = utils.RemoveContents(dockerDir)
	if err != nil {
		return trace.Wrap(err)
	}
	d.Infof("Wiped Docker data directory %v.", dockerDir)
	return nil
}

// PreCheck makes sure the phase runs on the correct node.
func (d *dockerDevicemapper) PreCheck(ctx context.Context) error {
	return trace.Wrap(d.Remote.CheckServer(ctx, d.Node))
}

// PostCheck is no-op.
func (*dockerDevicemapper) PostCheck(context.Context) error { return nil }

// dockerFormat is a phase executor that formats Docker device/partition
// with a filesystem suitable for overlay data.
type dockerFormat struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// Node is the node where Docker device should be formatted.
	Node storage.Server
	// Device is the absolute path to the Docker device, e.g. /dev/vdb.
	Device string
	// Remote allows to invoke remote commands.
	Remote fsm.Remote
}

// NewDockerFormat returns phase executor that formats Docker device/partition
// with a filesystem suitable for overlay data.
func NewDockerFormat(p fsm.ExecutorParams, remote fsm.Remote, log logrus.FieldLogger) (*dockerFormat, error) {
	node := *p.Phase.Data.Server
	return &dockerFormat{
		FieldLogger: log,
		Node:        node,
		Device:      getDockerDevice(p.Phase.Data),
		Remote:      remote,
	}, nil
}

// Execute creates filesystem on a Docker data device.
func (d *dockerFormat) Execute(ctx context.Context) error {
	d.Infof("Formatting device %v.", d.Device)
	filesystem, err := system.FormatDevice(ctx, system.FormatRequest{
		Path:  d.Device,
		Force: true,
		Log:   d.FieldLogger,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	d.Infof("Device %v formatted to %v.", d.Device, filesystem)
	return nil
}

// Rollback removes filesystem from the Docker data device.
func (d *dockerFormat) Rollback(ctx context.Context) error {
	d.Infof("Removing filesystem from %v.", d.Device)
	err := system.RemoveFilesystem(d.Device, d.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}
	d.Infof("Filesystem on %v erased.", d.Device)
	return nil
}

// PreCheck makes sure the phase runs on the correct node.
func (d *dockerFormat) PreCheck(ctx context.Context) error {
	return trace.Wrap(d.Remote.CheckServer(ctx, d.Node))
}

// PostCheck is no-op.
func (*dockerFormat) PostCheck(context.Context) error { return nil }

// dockerMount is a phase executor that mounts Docker device/partition
// to the node's Docker data directory.
type dockerMount struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// Node is node where Docker device should be mounted.
	Node storage.Server
	// Device is the absolute path to the Docker device, e.g. /dev/vdb.
	Device string
	// Remote allows to invoke remote commands.
	Remote fsm.Remote
}

// NewDockerMount returns phase executor that mounts Docker device/partition
// to the node's Docker data directory.
func NewDockerMount(p fsm.ExecutorParams, remote fsm.Remote, log logrus.FieldLogger) (*dockerMount, error) {
	node := *p.Phase.Data.Server
	return &dockerMount{
		FieldLogger: log,
		Node:        node,
		Device:      getDockerDevice(p.Phase.Data),
		Remote:      remote,
	}, nil
}

// Execute creates a systemd mount for Docker data directory.
func (d *dockerMount) Execute(ctx context.Context) error {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	filesystem, err := system.GetFilesystem(ctx, d.Device, utils.Runner)
	if err != nil {
		return trace.Wrap(err)
	}
	uuid, err := system.GetFilesystemUUID(ctx, d.Device, utils.Runner)
	if err != nil {
		return trace.Wrap(err)
	}
	config := mount.ServiceConfig{
		What:       storage.DeviceName(fmt.Sprintf("/dev/disk/by-uuid/%v", uuid)),
		Where:      filepath.Join(stateDir, defaults.PlanetDir, defaults.DockerDir),
		Filesystem: filesystem,
		Options:    []string{"defaults"},
	}
	d.Infof("Mounting %v to %v as %v.", config.What, config.Where, config.Filesystem)
	err = mount.Mount(config)
	if err != nil {
		return trace.Wrap(err)
	}
	d.Infof("Mount %v -> %v created and started.", config.What, config.Where)
	return nil
}

// Rollback removes the systemd mount for Docker data directory.
func (d *dockerMount) Rollback(ctx context.Context) error {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	config := mount.ServiceConfig{
		Where: filepath.Join(stateDir, defaults.PlanetDir, defaults.DockerDir),
	}
	d.Infof("Removing mount for %v.", config.Where)
	err = mount.Unmount(config.ServiceName())
	if err != nil {
		return trace.Wrap(err)
	}
	d.Infof("Mount %v removed.", config.Where)
	return nil
}

// PreCheck makes sure the phase runs on the correct node.
func (d *dockerMount) PreCheck(ctx context.Context) error {
	return trace.Wrap(d.Remote.CheckServer(ctx, d.Node))
}

// PostCheck is no-op.
func (*dockerMount) PostCheck(context.Context) error { return nil }

// getDockerDevice extracts Docker device path from the operation phase data.
func getDockerDevice(data *storage.OperationPhaseData) string {
	if data.Update != nil && data.Update.DockerDevice != "" {
		return data.Update.DockerDevice
	}
	return data.Server.Docker.Device.Path()
}
