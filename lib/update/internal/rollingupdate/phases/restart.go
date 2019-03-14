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
	"bytes"
	"context"
	"os/exec"

	libapp "github.com/gravitational/gravity/lib/app/service"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewRestart returns a new executor to restart the runtime container to apply
// the environment variables update
func NewRestart(
	params libfsm.ExecutorParams,
	operator localClusterGetter,
	operationID string,
	apps appGetter,
	packages, localPackages pack.PackageService,
	logger log.FieldLogger,
) (*restart, error) {
	if params.Phase.Data == nil || params.Phase.Data.Package == nil {
		return nil, trace.NotFound("no installed application package specified for phase %q",
			params.Phase.ID)
	}
	if params.Phase.Data.Update == nil || len(params.Phase.Data.Update.Servers) != 1 {
		return nil, trace.NotFound("no server specified for phase %q",
			params.Phase.ID)
	}
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &restart{
		FieldLogger:   logger,
		operationID:   operationID,
		packages:      packages,
		localPackages: localPackages,
		serviceUser:   cluster.ServiceUser,
		update:        params.Phase.Data.Update.Servers[0],
	}, nil
}

// Execute restarts the runtime container with the new configuration package
func (r *restart) Execute(ctx context.Context) error {
	err := r.pullUpdates()
	if err != nil {
		return trace.Wrap(err)
	}
	commands := [][]string{
		{"system", "update",
			"--changeset-id", r.operationID,
			"--with-status",
			"--debug",
			"--runtime-package", r.update.Runtime.Package.String(),
			"--runtime-config-package", r.update.Runtime.ConfigPackage.String(),
		},
	}
	for _, command := range commands {
		args := utils.Self(command...)
		cmd := exec.Command(args[0], args[1:]...)
		var buf bytes.Buffer
		err := utils.ExecL(cmd, &buf, r.FieldLogger)
		if err != nil {
			r.WithField("output", buf.String()).
				Warn("Failed to execute.")
			return trace.Wrap(err)
		}
	}
	return nil
}

// Rollback reverses the update and restarts the container with the old
// configuration package
func (r *restart) Rollback(context.Context) error {
	args := utils.Self("system", "rollback", "--changeset-id", r.operationID)
	cmd := exec.Command(args[0], args[1:]...)
	var buf bytes.Buffer
	err := utils.ExecL(cmd, &buf, r.FieldLogger)
	if err != nil {
		r.WithField("output", buf.String()).
			Warn("Failed to rollback.")
		return trace.Wrap(err)
	}
	return nil
}

// PreCheck is a no-op
func (*restart) PreCheck(context.Context) error {
	return nil
}

// PostCheck is a no-op
func (*restart) PostCheck(context.Context) error {
	return nil
}

func (r *restart) pullUpdates() error {
	updates := []loc.Locator{r.update.Runtime.Package, r.update.Runtime.ConfigPackage}
	for _, update := range updates {
		r.Infof("Pulling package update: %v.", update)
		_, err := libapp.PullPackage(libapp.PackagePullRequest{
			SrcPack: r.packages,
			DstPack: r.localPackages,
			Package: update,
		})
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

type restart struct {
	// FieldLogger specifies the logger for the phase
	log.FieldLogger
	packages      pack.PackageService
	localPackages pack.PackageService
	update        storage.ServerConfigUpdate
	serviceUser   storage.OSUser
	operationID   string
}
