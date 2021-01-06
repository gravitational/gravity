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

import { at } from 'lodash';
import cfg from 'oss-app/config';
import { formatPattern } from 'oss-app/lib/patternUtils';

cfg.init({

  routes: {
    // default app entry point
    defaultEntry: '/web/portal',

    // portal
    portalBase: '/web/portal',
    portalSettings: '/web/portal/settings',

    // settings
    settingsAuth: 'auth',
    settingsLicense: 'license',
    settingsRoles: 'roles'
  },

  modules: {
    site: {
      features: {
        remoteAccess: {
          enabled: true
        }
      }
    },

    settings: {
      opsCenterHeaderText:  'Gravity Ops Center',
      features: {
        licenseGenerator: {
          enabled: true
        }
      }
    },

    opsCenter: {
      headerText: 'Gravity Ops Center'
    },
  },

  api: {
    licenseGeneratorPath: '/portalapi/v1/license',
    oneTimeInstallLinkPath: '/portalapi/v1/tokens/install'
  },

  getInstallNewSiteOneTimeLinkRoute(name, repository, version, token){
    let baseUrl = window.location.origin;
    let route =  formatPattern(cfg.routes.installerNewSite, {name, repository, version});
    return `${baseUrl}${route}?install_token=${token}`
  },

  getOpsCenterHeaderText(){
    let [headerText] = at(cfg, 'modules.opsCenter.headerText');
    return headerText;
  },

  getSettingsOpsCenterHeaderText(){
    let [headerText] = at(cfg, 'modules.settings.opsCenterHeaderText');
    return headerText;
  },

  isSettingsLicenseGenEnabled() {
    let [isLicenseGenEnabled] = at(cfg, 'modules.settings.features.licenseGenerator.enabled');
    return isLicenseGenEnabled;
  }

})

export default cfg;
