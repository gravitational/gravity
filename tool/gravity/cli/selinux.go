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
	"os"
	"syscall"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	libselinux "github.com/gravitational/gravity/lib/system/selinux"

	"github.com/gravitational/trace"
	"github.com/opencontainers/selinux/go-selinux"
)

// BootstrapSELinuxAndRespawn prepares the node for the installation with SELinux support
// and restarts the process under the proper SELinux context if necessary
func BootstrapSELinuxAndRespawn(config libselinux.BootstrapConfig) error {
	if !selinux.GetEnabled() {
		return nil
	}
	label, err := selinux.CurrentLabel()
	log.WithField("label", label).Info("Current process label.")
	if err != nil {
		return trace.Wrap(err)
	}
	procContext, err := selinux.NewContext(label)
	if err != nil {
		return trace.Wrap(err)
	}
	if procContext["type"] == libselinux.GravityInstallerProcessContext["type"] {
		if config.Force {
			if err := libselinux.Bootstrap(config); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}
	if err := libselinux.Bootstrap(config); err != nil {
		return trace.Wrap(err)
	}
	newProcContext := libselinux.MustNewContext(label)
	newProcContext["type"] = libselinux.GravityInstallerProcessContext["type"]
	log.WithField("context", newProcContext).Info("Set process context.")
	if err := selinux.SetExecLabel(newProcContext.Get()); err != nil {
		return trace.Wrap(err)
	}
	log.WithField("args", os.Args).Info("Respawn.")
	cmd := os.Args[0]
	return syscall.Exec(cmd, os.Args, nil)
}

func bootstrapSelinux(env *localenv.LocalEnvironment, path string, vxlanPort int) error {
	config := libselinux.BootstrapConfig{
		VxlanPort: vxlanPort,
	}
	if path == "" {
		return libselinux.Bootstrap(config)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, defaults.SharedReadMask)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	return libselinux.WriteBootstrapScript(f, config)
}
