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
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// CheckSiteStatus runs application status hook and updates cluster status appropriately
func (o *Operator) CheckSiteStatus(key ops.SiteKey) error {
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
	reason := storage.ReasonNodeDegraded
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
		return trace.Wrap(statusErr)
	}

	// all status checks passed so if the cluster was previously disabled
	// because of those checks, enable it back
	if cluster.backendSite.State == ops.SiteStateDegraded {
		if cluster.backendSite.Reason != storage.ReasonLicenseInvalid {
			err := o.ActivateSite(ops.ActivateSiteRequest{
				AccountID:  key.AccountID,
				SiteDomain: cluster.backendSite.Domain,
			})
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// checkPlanetStatus checks the cluster health using planet agents
func (s *site) checkPlanetStatus(ctx context.Context) error {
	planetStatus, err := status.FromPlanetAgent(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, node := range planetStatus.Nodes {
		if node.AdvertiseIP != "" && node.Status != status.NodeHealthy {
			return trace.BadParameter("node %v is not healthy: %#v",
				node.AdvertiseIP, planetStatus)
		}
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
		defer func() {
			err := s.service.cfg.Apps.DeleteAppHookJob(ctx, *ref)
			if err != nil {
				s.Warnf("Failed to delete status hook %v: %v.",
					ref, trace.DebugReport(err))
			}
		}()
	}
	if err != nil {
		return trace.Wrap(err, "status hook failed: %s", out)
	}
	return nil
}
