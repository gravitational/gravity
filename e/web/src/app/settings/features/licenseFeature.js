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

// oss imports
import FeatureBase from 'oss-app/modules/featureBase'
import htmlUtils from 'oss-app/lib/htmlUtils';
import { addNavItem } from 'oss-app/modules/settings/flux/actions';
import { NavGroupEnum } from 'oss-app/modules/settings/enums';

// local imports
import LicenseGenerator from './../components/licenseGen/licenseGenerator';
import * as featureFlags from './featureFlags';
import { initLicenseGen } from './../flux/license/actions';
import cfg from './../../config';

class LicenseFeature extends FeatureBase {

  constructor(routes) {
    super()
    routes.push({
      path: this.getIndexRoute(),
      onEnter: initLicenseGen,
      component: super.withMe(LicenseGenerator)
    });
  }

  getIndexRoute(){
    return cfg.routes.settingsLicense;
  }

  onload(context) {
    const { baseUrl } = context;
    const enabled = featureFlags.settingsLicense();
    const navItem = {
      icon: 'fa fa-key',
      title: 'Licenses',
      to: htmlUtils.joinPaths(baseUrl, this.getIndexRoute())
    }

    if (enabled) {
      addNavItem(NavGroupEnum.APPLICATION, navItem);
    }
  }
}

export default LicenseFeature;