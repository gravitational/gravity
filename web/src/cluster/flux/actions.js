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

import cfg from 'app/config';
import { Activator } from 'oss-app/lib/featureBase';
import $ from 'jQuery';
import { fetchUserContext } from 'app/flux/user/actions';
import * as featureFlags from 'app/cluster/featureFlags';
import { setCluster } from 'app/flux/cluster/actions';
import { fetchRemoteAccess, fetchSiteInfo } from './info/actions';
import service, { applyConfig } from 'app/services/clusters';
import { setReleases } from './apps/actions';

export function initCluster(siteId, features) {
  cfg.setDefaultSiteId(siteId);
  return fetchUserContext().then(() => init(features));
}

function init(features){
  return $.when(
    service.fetchCluster({shallow: false}),
    fetchSiteInfo(),
    fetchRemoteAccess(),
  )
  .then((...responses) => {
    const [ cluster ] = responses;
    // Apply cluster web config settings
    applyConfig(cluster);

    // Init cluster store
    setCluster(cluster);

    // Init releases store
    setReleases(cluster.apps);

    // Initialize features
    const activator = new Activator(features);
    activator.onload({ featureFlags });
  })
}