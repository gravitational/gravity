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

package environ

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system"
	"github.com/gravitational/gravity/lib/system/mount"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ConfigureStateDirectory sets up the state directory stateDir
// on host.
// Optional devicePath specifies the device dedicated for state.
// If the device has been specified, it will be formatted and mounted
// as the state directory.
func ConfigureStateDirectory(stateDir, devicePath string) (err error) {
	_, err = utils.StatDir(stateDir)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	devicePath = storage.DeviceName(devicePath).Path()
	if devicePath == "" {
		err = os.MkdirAll(stateDir, defaults.SharedDirMask)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		return nil
	}

	_, err = os.Stat(devicePath)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to query device at %q", devicePath)
	}

	// Even if the directory exists, mount it on the specified device.
	// If this is not possible, the operation will fail as expected.
	var filesystem string
	filesystem, err = formatDevice(devicePath)
	if err != nil {
		return trace.Wrap(err)
	}

	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	config := mount.ServiceConfig{
		What:       storage.DeviceName(devicePath),
		Where:      stateDir,
		Filesystem: filesystem,
		Options:    []string{"defaults"},
	}
	err = mount.MountService(config, defaults.GravityMountService, services)
	if err != nil {
		return trace.Wrap(err, "failed to mount %q on %q", stateDir, devicePath)
	}

	return nil
}

// GetServiceName returns the name of the service configured in the specified state directory stateDir
func GetServiceName(stateDir string) (name string, err error) {
	path, err := GetServicePath(stateDir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Base(path), nil
}

// GetServicePath returns the path of the service configured in the specified state directory stateDir
func GetServicePath(stateDir string) (path string, err error) {
	socketPath, err := GetSocketPath(stateDir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return GetServicePathFromSocketPath(socketPath), nil
}

// GetServicePathFromSocketPath returns the path of the service given the socket path
func GetServicePathFromSocketPath(socketPath string) (path string) {
	serviceName := strings.TrimSuffix(filepath.Base(socketPath), filepath.Ext(socketPath))
	return defaults.SystemUnitPath(systemservice.FullServiceName(serviceName))
}

// GetSocketPath returns the path of the socket from the specified state directory
func GetSocketPath(stateDir string) (path string, err error) {
	for _, name := range []string{
		defaults.GravityRPCInstallerSocketName,
		defaults.GravityRPCAgentSocketName,
	} {
		if ok, _ := utils.IsFile(filepath.Join(stateDir, name)); ok {
			return filepath.Join(stateDir, name), nil
		}
	}
	return "", trace.NotFound("no installer socket file in %v", stateDir)
}

func formatDevice(path string) (filesystem string, err error) {
	type formatter struct {
		fsType string
		args   []string
	}
	formatters := []formatter{
		{"xfs", []string{"mkfs.xfs", "-f"}},
		{"ext4", []string{"mkfs.ext4", "-F"}},
	}

	filesystem, err = system.GetFilesystem(context.TODO(), path, utils.Runner)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if filesystem != "" {
		log.Infof("File system on %q is %v.", path, filesystem)
		return filesystem, nil
	}

	// format the device if the specified device does not have a file system yet
	log.Infof("Device %q has no file system.", path)

	var fmt formatter
	var out bytes.Buffer
	for _, fmt = range formatters {
		out.Reset()
		args := append(fmt.args, path)
		log.Debugf("Formatting %q as %v.", path, fmt.fsType)
		cmd := exec.Command(args[0], args[1:]...)
		if err = utils.ExecL(cmd, &out, log.StandardLogger()); err != nil {
			log.Warnf("Failed to format %q as %q: %v (%v).",
				path, fmt.fsType, out.String(), err)
		}
		if err == nil {
			filesystem = fmt.fsType
			break
		}
	}
	if err != nil {
		return "", trace.Wrap(err, "failed to format %q as %q: %v",
			path, fmt.fsType, out.String())
	}
	return filesystem, nil
}
