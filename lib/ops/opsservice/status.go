/*
Copyright 2018 Gravitational, Inc.

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

package opsservice

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

// CheckSiteStatus runs application status hook and updates cluster status appropriately
func (o *Operator) CheckSiteStatus(ctx context.Context, key ops.SiteKey) error {
	cluster, err := o.openSite(key)
	if err != nil {
		return trace.Wrap(err)
	}

	// pause status checks while the cluster is undergoing an operation
	switch cluster.backendSite.State {
	case ops.SiteStateActive, ops.SiteStateDegraded:
	default:
		o.Infof("Status checks are paused, cluster is %v.",
			cluster.backendSite.State)
		return nil
	}

	statusErr := cluster.checkPlanetStatus(context.TODO())
	reason := storage.ReasonClusterDegraded
	if statusErr == nil {
		statusErr = cluster.checkStatusHook(context.TODO())
		reason = storage.ReasonStatusCheckFailed
	}

	if statusErr != nil {
		err := o.DeactivateSite(ops.DeactivateSiteRequest{
			AccountID:  key.AccountID,
			SiteDomain: cluster.backendSite.Domain,
			Reason:     reason,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		events.Emit(ctx, o, events.ClusterUnhealthy, events.Fields{
			events.FieldReason: reason,
		})
		return trace.Wrap(statusErr)
	}

	// all status checks passed so if the cluster was previously disabled
	// because of those checks, enable it back
	if cluster.canActivate() {
		err := o.ActivateSite(ops.ActivateSiteRequest{
			AccountID:  key.AccountID,
			SiteDomain: cluster.backendSite.Domain,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		events.Emit(ctx, o, events.ClusterHealthy)
	}

	return nil
}

// canActivate retursn true if the cluster is disabled b/c of status checks
func (s *site) canActivate() bool {
	return s.backendSite.State == ops.SiteStateDegraded &&
		s.backendSite.Reason != storage.ReasonLicenseInvalid
}

// checkPlanetStatus checks the cluster health using planet agents
func (s *site) checkPlanetStatus(ctx context.Context) error {
	planetStatus, err := status.FromPlanetAgent(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	if planetStatus.GetSystemStatus() != agentpb.SystemStatus_Running {
		return trace.BadParameter("cluster is not healthy: %#v", planetStatus)
	}
	return nil
}

// checkStatusHook executes the application's status hook
func (s *site) checkStatusHook(ctx context.Context) error {
	if !s.app.Manifest.HasHook(schema.HookStatus) {
		s.Debugf("Application %s does not have status hook.", s.app)
		return nil
	}
	ref, out, err := app.RunAppHook(ctx, s.service.cfg.Apps, app.HookRunRequest{
		Application: s.backendSite.App.Locator(),
		Hook:        schema.HookStatus,
		ServiceUser: s.serviceUser(),
	})
	if ref != nil {
		err := s.service.cfg.Apps.DeleteAppHookJob(ctx, app.DeleteAppHookJobRequest{
			HookRef: *ref,
			Cascade: true,
		})
		if err != nil {
			s.Warnf("Failed to delete status hook %v: %v.",
				ref, trace.DebugReport(err))
		}
	}
	if err != nil {
		return trace.Wrap(err, "status hook failed: %s", out)
	}
	return nil
}
