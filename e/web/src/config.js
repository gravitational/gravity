/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import cfg from 'oss-app/config';
import { generatePath } from "react-router";

cfg.init({

  isEnterprise: true,

  routes: {
    // cluster
    clusterRoles: '/web/site/:siteId/roles',
    clusterAuthConnectors: '/web/site/:siteId/auth',

    // default app entry point
    defaultEntry: '/web/portal',

    // hub
    hubBase: '/web/portal',
    hubClusters: '/web/portal/clusters',
    hubLicenses: '/web/portal/licenses',
    hubCatalog: '/web/portal/catalog',
    hubSettings: '/web/portal/settings',
    hubSettingCert: '/web/portal/settings/cert',
    hubSettingAccount: '/web/portal/settings/account',
    hubAccess: '/web/portal/access',
    hubAccessRoles: '/web/portal/access/roles',
    hubAccessUsers: '/web/portal/access/users',
    hubAccessAuth: '/web/portal/access/auth',
  },

  api: {
    licenseGeneratorPath: '/portalapi/v1/license',
  },

  getClusterRolesRoute(siteId){
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.clusterRoles, { siteId });
  },

  getClusterAuthConnectorsRoute(siteId){
    siteId = siteId || cfg.defaultSiteId;
    return generatePath(cfg.routes.clusterAuthConnectors, { siteId })
  },
});

export default cfg;