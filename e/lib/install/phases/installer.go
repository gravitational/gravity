// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/client"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/webpack"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewInstaller returns a new "installer" phase
func NewInstaller(p fsm.ExecutorParams, operator ops.Operator, wizardPack pack.PackageService, wizardApps app.Applications) (*installerExecutor, error) {
	// TODO pass insecure flag
	httpClient := httplib.GetClient(true)
	opsPack, err := webpack.NewBearerClient(p.Phase.Data.Agent.OpsCenterURL,
		p.Phase.Data.Agent.Password, roundtrip.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opsApps, err := client.NewBearerClient(p.Phase.Data.Agent.OpsCenterURL,
		p.Phase.Data.Agent.Password, client.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
	}
	return &installerExecutor{
		FieldLogger:    logger,
		WizardPackages: wizardPack,
		WizardApps:     wizardApps,
		OpsPackages:    opsPack,
		OpsApps:        opsApps,
		ExecutorParams: p,
	}, nil
}

type installerExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// WizardPackages is the installer process pack service
	WizardPackages pack.PackageService
	// WizardApps is the installer process app service
	WizardApps app.Applications
	// OpsPackages is the remote Ops Center pack service
	OpsPackages pack.PackageService
	// OpsApps is the remote Ops Center app service
	OpsApps app.Applications
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute executes the installer phase
func (p *installerExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Downloading cluster image from Gravity Hub")
	p.Info("Downloading cluster image from Gravity Hub.")
	puller := app.Puller{
		FieldLogger: p.FieldLogger,
		SrcPack:     p.OpsPackages,
		DstPack:     p.WizardPackages,
		SrcApp:      p.OpsApps,
		DstApp:      p.WizardApps,
		Upsert:      true,
	}
	err := puller.PullApp(ctx, *p.Phase.Data.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Info("Cluster image has been downloaded.")
	return nil
}

// Rollback is no-op for this phase
func (*installerExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*installerExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*installerExecutor) PostCheck(ctx context.Context) error {
	return nil
}
