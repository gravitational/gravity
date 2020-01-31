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

package phases

import (
	"bytes"
	"context"
	"os/exec"

	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/update"
	libbasephase "github.com/gravitational/gravity/lib/update/internal/rollingupdate/phases"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewRestart returns a new executor to restart the runtime container to apply
// the environment variables update
func NewRestart(
	params libfsm.ExecutorParams,
	operator libbasephase.LocalClusterGetter,
	operation ops.SiteOperation,
	apps appGetter,
	backend storage.Backend,
	packages pack.PackageService,
	localPackages update.LocalPackageService,
	logger log.FieldLogger,
) (*restart, error) {
	base, err := libbasephase.NewRestart(params, operator, operation.ID, apps, backend, packages, localPackages, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := clusterconfig.Unmarshal(operation.UpdateConfig.Config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &restart{
		FieldLogger: logger,
		config:      *config,
		base:        base,
	}, nil
}

// Execute restarts the runtime container with the new configuration package
func (r *restart) Execute(ctx context.Context) error {
	if config := r.config.GetGlobalConfig(); shouldUpdatePodCIDR(config) {
		if err := r.removeCNIBridge(); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(r.base.Execute(ctx))
}

// Rollback reverses the update and restarts the container with the old
// configuration package
func (r *restart) Rollback(ctx context.Context) error {
	// cni0 bridge should be recreated on kubelet restart
	// when the flannel->bridge cni plugin executes ADD operation
	// for the next container
	if config := r.config.GetGlobalConfig(); shouldUpdatePodCIDR(config) {
		if err := r.removeCNIBridge(); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(r.base.Rollback(ctx))
}

// PreCheck is a no-op
func (*restart) PreCheck(context.Context) error {
	return nil
}

// PostCheck is a no-op
func (*restart) PostCheck(context.Context) error {
	return nil
}

func (r *restart) removeCNIBridge() error {
	exists, err := linkExists(cniBridge)
	if exists {
		r.Info("Removing network interface cni0")
		return linkDel(cniBridge)
	}
	if err != nil {
		r.WithError(err).Warn("Failed to determine whether cni0 bridge exists.")
	}
	// Nothing to do
	return nil
}

type restart struct {
	// FieldLogger specifies the logger for the phase
	log.FieldLogger
	config clusterconfig.Resource
	base   libfsm.PhaseExecutor
}

func shouldUpdatePodCIDR(config clusterconfig.Global) bool {
	return config.PodCIDR != ""
}

func linkDel(name string) error {
	out, err := command("ip", "link", "del", name).CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(out))
	}
	return nil
}

func linkExists(name string) (exists bool, err error) {
	var buf bytes.Buffer
	cmd := command("ip", "link", "show", name)
	cmd.Stderr = &buf
	err = cmd.Run()
	if err == nil {
		return true, nil
	}
	if utils.ExitStatusFromError(err) == nil {
		return false, trace.Wrap(err, buf.String())
	}
	log.WithError(err).Warnf("Failed to find link device for %q.", name)
	// Failed to match an existing link device
	return false, nil
}

func command(args ...string) *exec.Cmd {
	args = utils.Exe.PlanetCommandSlice(args)
	return exec.Command(args[0], args[1:]...)
}

const cniBridge = "cni0"
