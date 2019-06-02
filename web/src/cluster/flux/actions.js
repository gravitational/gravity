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

import { SiteStateEnum } from 'app/services/enums';
import cfg from 'app/config';
import $ from 'jQuery';
import * as featureFlags from 'app/cluster/featureFlags';
import { setCluster } from 'app/flux/cluster/actions';
import { fetchRemoteAccess, fetchSiteInfo } from './info/actions';
import service, { applyConfig } from 'app/services/clusters';
import { fetchNodes } from './nodes/actions';
import { setReleases } from './apps/actions';

export function initCluster(siteId, featureActivator) {
  cfg.setDefaultSiteId(siteId);
  return $.when(
    service.fetchCluster({shallow: false}),
    fetchNodes(),
    fetchSiteInfo(),
    fetchRemoteAccess(),
  )
  .then((...responses) => {
    const [ cluster ] = responses;

    if (cluster.state === SiteStateEnum.UNINSTALLING) {
      handleUninstallState(siteId);
      return;
    }

    // Apply cluster web config settings
    applyConfig(cluster);

    // Init cluster store
    setCluster(cluster);

    // Init releases store
    setReleases(cluster.apps);

    // Initialize features
    featureActivator.onload({ siteId, featureFlags });
  })
}

function handleUninstallState(siteId){
  // Redirect to cluster uinstall screen
  const url = cfg.getSiteUninstallRoute(siteId);
  history.push(url, true);
}
