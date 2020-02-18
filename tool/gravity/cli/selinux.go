/*
Copyright 2020 Gravitational, Inc.

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
	"fmt"
	"os"
	"syscall"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	libselinux "github.com/gravitational/gravity/lib/system/selinux"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/opencontainers/selinux/go-selinux"
)

// BootstrapSELinuxAndRespawn prepares the node for the installation with SELinux support
// and restarts the process under the proper SELinux context if necessary
func BootstrapSELinuxAndRespawn(ctx context.Context, config libselinux.BootstrapConfig, printer utils.Printer) error {
	if !selinux.GetEnabled() {
		return nil
	}
	logger := log.WithField(trace.Component, "selinux")
	label, err := selinux.CurrentLabel()
	logger.WithField("label", label).Info("Current process label.")
	if err != nil {
		return trace.Wrap(err)
	}
	procContext, err := selinux.NewContext(label)
	if err != nil {
		return trace.Wrap(err)
	}
	if !isSELinuxAlreadyBootstrapped() {
		printer.PrintStep("Bootstrapping installer for SELinux")
		if err := libselinux.Bootstrap(ctx, config); err != nil {
			return trace.Wrap(err)
		}
	}
	if procContext["type"] == libselinux.GravityInstallerProcessContext["type"] {
		// Already running in the expected SELinux domain
		return nil
	}
	newProcContext := libselinux.MustNewContext(label)
	newProcContext["type"] = libselinux.GravityInstallerProcessContext["type"]
	logger.WithField("context", newProcContext).Info("Set process context.")
	if err := selinux.SetExecLabel(newProcContext.Get()); err != nil {
		return trace.Wrap(err)
	}
	logger.WithField("args", os.Args).Info("Respawn.")
	cmd := os.Args[0]
	return syscall.Exec(cmd, os.Args, newRespawnEnviron())
}

func bootstrapSELinux(env *localenv.LocalEnvironment, path, stateDir string, vxlanPort int) error {
	config := libselinux.BootstrapConfig{
		StateDir: stateDir,
	}
	if path == "" {
		return libselinux.Bootstrap(context.TODO(), config)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, defaults.SharedReadMask)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	return libselinux.WriteBootstrapScript(f, config)
}

func isSELinuxAlreadyBootstrapped() bool {
	_, ok := os.LookupEnv(alreadyBootstrappedEnv)
	return ok
}

func newRespawnEnviron() (environ []string) {
	return append(os.Environ(), fmt.Sprintf("%v=yes", alreadyBootstrappedEnv))
}

const alreadyBootstrappedEnv = "GRAVITY_SELINUX_BOOTSTRAPPED"
