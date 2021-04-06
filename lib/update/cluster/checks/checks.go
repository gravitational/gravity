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

package checks

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// CheckerConfig is the configuration for upgrade pre-flight checker.
type CheckerConfig struct {
	// ClusterOperator is the operator service of the installed cluster.
	ClusterOperator ops.Operator
	// ClusterApps is the app service of the installed cluster.
	ClusterApps app.Applications
	// UpgradeApps is the app service containing upgrade image.
	UpgradeApps app.Applications
	// UpgradePackage is the upgrade image locator.
	UpgradePackage loc.Locator
	// Agents is the agent service interface (agents must be already deployed).
	Agents rpc.AgentRepository
}

// NewChecker creates a checker for validating requirements of the
// upgrade operation.
func NewChecker(ctx context.Context, config CheckerConfig) (checks.Checker, error) {
	cluster, err := config.ClusterOperator.GetLocalSite(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installed, err := config.ClusterApps.GetApp(cluster.App.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	upgrade, err := config.UpgradeApps.GetApp(config.UpgradePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if installed.Name() != upgrade.Name() {
		return nil, trace.BadParameter("invalid upgrade image %q: %q is installed",
			upgrade.Name(), installed.Name())
	}
	dockerConfig := storage.DockerConfig{
		StorageDriver: cluster.ClusterState.Docker.StorageDriver,
	}
	if upgrade.Manifest.SystemDocker().StorageDriver != "" {
		dockerConfig.StorageDriver = upgrade.Manifest.SystemDocker().StorageDriver
	}
	nodes, err := checks.GetServers(ctx, config.Agents, cluster.ClusterState.Servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqs, err := checks.RequirementsFromManifests(installed.Manifest, upgrade.Manifest,
		cluster.ClusterState.Servers.Profiles(), dockerConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := checks.New(checks.Config{
		Remote:       checks.NewRemote(config.Agents),
		Manifest:     upgrade.Manifest,
		Servers:      nodes,
		Requirements: reqs,
		Features: checks.Features{
			TestPorts: true,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return checker, nil
}
