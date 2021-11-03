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
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewPushAppToRegistry returns a new phase executor
func NewPushAppToRegistry(ctx context.Context, p fsm.ExecutorParams, operator ops.Operator, apps libapp.Applications) (fsm.PhaseExecutor, error) {
	cluster, err := operator.GetLocalSite(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pushAppToRegistry{
		apps:       apps,
		clusterApp: cluster.App.Package,
	}, nil
}

// Execute pushes the cluster application images to the local registry
func (p *pushAppToRegistry) Execute(ctx context.Context) error {
	ctx, cancel := defaults.WithTimeout(ctx)
	defer cancel()
	err := utils.RetryTransient(ctx, utils.NewUnlimitedExponentialBackOff(), func() error {
		return p.apps.ExportApp(libapp.ExportAppRequest{
			Package:         p.clusterApp,
			RegistryAddress: defaults.LocalRegistryAddr,
			CertName:        defaults.DockerRegistry,
		})
	})
	return trace.Wrap(err)
}

// PreCheck is a noop for this executor
func (*pushAppToRegistry) PreCheck(ctx context.Context) error { return nil }

// PostCheck is a noop for this executor
func (*pushAppToRegistry) PostCheck(ctx context.Context) error { return nil }

// Rollback is a noop for this executor
func (*pushAppToRegistry) Rollback(ctx context.Context) error { return nil }

type pushAppToRegistry struct {
	logrus.FieldLogger
	apps       libapp.Applications
	clusterApp loc.Locator
}
