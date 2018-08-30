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
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// CheckSiteStatus runs app status hook and updates site status appropriately.
func (o *Operator) CheckSiteStatus(key ops.SiteKey) error {
	cluster, err := o.openSite(key)
	if err != nil {
		return trace.Wrap(err)
	}

	if !cluster.app.Manifest.HasHook(schema.HookStatus) {
		log.Debugf("%v does not have status hook", key)
		return nil
	}

	ref, out, err := app.RunAppHook(context.TODO(), o.cfg.Apps, app.HookRunRequest{
		Application: cluster.backendSite.App.Locator(),
		Hook:        schema.HookStatus,
		ServiceUser: cluster.serviceUser(),
	})

	if ref != nil {
		defer func() {
			err := o.cfg.Apps.DeleteAppHookJob(context.TODO(), *ref)
			if err != nil {
				log.Warningf("failed to delete hook %v: %v",
					ref, trace.DebugReport(err))
			}
		}()
	}

	if err != nil {
		req := ops.DeactivateSiteRequest{
			AccountID:  key.AccountID,
			SiteDomain: cluster.backendSite.Domain,
			Reason:     storage.ReasonStatusCheckFailed,
		}
		if err := o.DeactivateSite(req); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(err, string(out))
	}

	if cluster.backendSite.State == ops.SiteStateDegraded && cluster.backendSite.Reason == storage.ReasonStatusCheckFailed {
		req := ops.ActivateSiteRequest{
			AccountID:  key.AccountID,
			SiteDomain: cluster.backendSite.Domain,
		}
		if err := o.ActivateSite(req); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
