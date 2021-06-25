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
	"context"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/pack"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewClusterPackages returns executor that updates packages in the cluster state.
//
// Specifically, it updates the certificate authority package stored in the
// cluster state with the one newly generated during the configure phase from
// the local state.
func NewClusterPackages(p fsm.ExecutorParams, operator ops.Operator, localPackages pack.PackageService) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	operation, err := operator.GetSiteOperation(p.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clusterPackagesExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Operation:      *operation,
		LocalPackages:  localPackages,
	}, nil
}

type clusterPackagesExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
	// Operation is the current reconfigure operation.
	Operation ops.SiteOperation
	// LocalPackages is the node-local package service.
	LocalPackages pack.PackageService
}

// Execute updates the certificate authority package in the cluster state.
func (p *clusterPackagesExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Updating cluster packages")
	clusterPackages, err := localenv.ClusterPackages()
	if err != nil {
		return trace.Wrap(err)
	}
	caPackage := opsservice.PlanetCertAuthorityPackage(p.Plan.ClusterName)
	_, err = service.PullPackage(service.PackagePullRequest{
		SrcPack: p.LocalPackages,
		DstPack: clusterPackages,
		Package: caPackage,
		Upsert:  true,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debugf("Updated %v in the cluster state.", caPackage)
	return nil
}

// Rollback is no-op for this phase.
func (*clusterPackagesExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*clusterPackagesExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*clusterPackagesExecutor) PostCheck(ctx context.Context) error {
	return nil
}
