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

package cli

import (
	"context"
	"os"
	"os/exec"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/system/mount"
	"github.com/gravitational/gravity/lib/system/selinux"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func policyInstall(env *localenv.LocalEnvironment, addr string) error {
	// TODO
	_, err := env.PackageServiceWithOptions(nil)
	if err != nil {
		return trace.Wrap(err)
	}
	// m := selinux.NewPolicyManager(clusterPackages)
	// return m.Install()
	return nil
}

func bootstrapSelinux(env *localenv.LocalEnvironment, path string) error {
	if path == "" {
		return selinux.Bootstrap(utils.Exe.WorkingDir)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, defaults.SharedExecutableMask)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	return selinux.WriteBootstrapScript(f, utils.Exe.WorkingDir)
}

func restoreFilecontexts(env *localenv.LocalEnvironment, rootfsDir string) error {
	logger := log.WithField("rootfs", rootfsDir)
	mounts := []string{
		"/etc/selinux",
		"/sys/fs/selinux",
	}
	m := mount.NewMounter(rootfsDir)
	for _, mount := range mounts {
		if err := m.BindMount(mount, mount); err != nil {
			return trace.Wrap(err, "failed to mount %v", mount)
		}
	}
	defer func() {
		for _, mount := range mounts {
			if err := m.Unmount(mount); err != nil {
				logger.WithFields(logrus.Fields{
					logrus.ErrorKey: err,
					"dir":           mount,
				}).Warn("Failed to unmount.")
			}
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.WatchTerminationSignals(ctx, cancel, env)
	defer interrupt.Close()

	args := []string{
		"system", "exec-jail", "--path", rootfsDir,
		defaults.RestoreconBin,
		"-R",
		"-vvv",
		"-i",
		"-e", "/etc/selinux",
		"-e", "/sys/fs/selinux",
		"-0",
		"-f", "/.relabelpaths",
	}
	args = utils.Self(args...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	logger.WithFields(logrus.Fields{
		logrus.ErrorKey: err,
		"rootfs":        rootfsDir,
	}).Info("Restore file contexts in rootfs.")
	return trace.Wrap(err)
}
