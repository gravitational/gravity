/*
Copyright 2021 Gravitational, Inc.

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

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	installphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// NewExport returns a new phase executor for exporting the cluster application
// into the local node's registry
func NewExport(ctx context.Context, p fsm.ExecutorParams, operator ops.Operator,
	clusterPackages, localPackages pack.PackageService,
	clusterApps, localApps libapp.Applications, remote fsm.Remote,
) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase:       p.Phase.ID,
			constants.FieldAdvertiseIP: p.Phase.Data.Server.AdvertiseIP,
			constants.FieldHostname:    p.Phase.Data.Server.Hostname,
			"package":                  p.Phase.Data.Package,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	exec, err := installphases.NewExport(p, operator, localPackages, localApps, remote)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &exportExecutor{
		FieldLogger:     logger,
		exec:            exec,
		clusterPackages: clusterPackages,
		clusterApps:     clusterApps,
		localPackages:   localPackages,
		localApps:       localApps,
		clusterApp:      *p.Phase.Data.Package,
	}, nil
}

// Execute pushes the cluster application images to the local registry
func (p *exportExecutor) Execute(ctx context.Context) error {
	ctx, cancel := defaults.WithTimeout(ctx)
	defer cancel()
	puller := libapp.Puller{
		FieldLogger: p.FieldLogger,
		SrcPack:     p.clusterPackages,
		DstPack:     p.localPackages,
		SrcApp:      p.clusterApps,
		DstApp:      p.localApps,
		// Do not pull over existing packages
		OnConflict: libapp.OnConflictContinue,
	}
	err := puller.PullApp(ctx, p.clusterApp)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := p.exec.Execute(ctx); err != nil {
		return trace.Wrap(err)
	}
	return p.localPackages.DeletePackage(p.clusterApp)
}

// PreCheck is a noop for this executor
func (*exportExecutor) PreCheck(ctx context.Context) error { return nil }

// PostCheck is a noop for this executor
func (*exportExecutor) PostCheck(ctx context.Context) error { return nil }

// Rollback is a noop for this executor
func (*exportExecutor) Rollback(ctx context.Context) error { return nil }

type exportExecutor struct {
	logrus.FieldLogger

	exec            *installphases.ExportExecutor
	clusterPackages pack.PackageService
	clusterApps     libapp.Applications
	localPackages   pack.PackageService
	clusterApp      loc.Locator
	localApps       libapp.Applications
}
