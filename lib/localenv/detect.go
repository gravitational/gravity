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

package localenv

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
)

// DetectCluster attempts to detect whether there is a deployed cluster.
//
// The behavior is as follows:
//
//   - If the cluster is deployed/healthy, returns nil.
//   - If the cluster is not deployed and the node's clean, returns NotFound.
//   - If partial state is detected, returns a specific error.
func DetectCluster(ctx context.Context, env *LocalEnvironment) error {
	// If the local cluster checks succeed, we have a cluster.
	clusterErr := checkCluster(ctx, env)
	if clusterErr == nil {
		return nil
	}
	// Otherwise see if there is a partial state by looking at local packages.
	err := checkLocalPackages(env)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("no cluster detected")
		}
		return trace.NewAggregate(err, clusterErr)
	}
	// There are local packages: assume partial installation state or
	// degraded state and log the original error for troubleshooting.
	log.WithError(clusterErr).Warn("Failed to query local cluster.")
	return trace.BadParameter(`Detected local Gravity state on the node but the cluster is inaccessible.

This usually means one of two things:

* The cluster is partially installed.
* The cluster is degraded.

To clean up the node from the partial state, run "gravity leave --force".
Otherwise, run "gravity status" and troubleshoot the degraded cluster.`)
}

// checkCluster performs a number of checks to see if there is a running
// local cluster.
//
// Returns nil if the cluster is deployed and a specific error otherwise.
func checkCluster(ctx context.Context, env *LocalEnvironment) error {
	// 1. Try to get operator client for the local cluster.
	clusterOperator, err := env.SiteOperator(httplib.WithTimeout(time.Second))
	if err != nil {
		return trace.Wrap(err, "failed to get local cluster operator")
	}
	// 2. Try to make a request to the cluster controller (gravity-site).
	cluster, err := clusterOperator.GetLocalSite(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to query local cluster")
	}
	// 3. See if the cluster has actually been installed successfully.
	//
	// TODO(r0mant): Currently this will actually always find completed
	// install operation because it is always set to "completed" state
	// when gravity-site comes up for the first time. It will be fixed
	// when https://github.com/gravitational/gravity/issues/856 is fixed.
	_, err = ops.GetCompletedInstallOperation(cluster.Key(), clusterOperator)
	if err != nil {
		return trace.Wrap(err, "failed to find completed install operation")
	}
	// At this point we should be pretty confident that the cluster is
	// up and running.
	return nil
}

// checkLocalPackages checks whether the node has any packages in the local state.
//
// Returns nil if there are packages and NotFound otherwise.
func checkLocalPackages(env *LocalEnvironment) error {
	packages, err := env.Packages.GetPackages(defaults.SystemAccountOrg)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(packages) == 0 {
		return trace.NotFound("no local packages")
	}
	return nil
}
