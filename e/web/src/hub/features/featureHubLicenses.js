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

import withFeature, { FeatureBase } from 'oss-app/components/withFeature';
import cfg from 'e-app/config'
import Licenses from './../components/HubLicenses';
import { addTopNavItem } from './../flux/nav/actions';

class LicensesFeature extends FeatureBase {

  constructor(){
    super();
    this.Component = withFeature(this)(Licenses);
  }

  getRoute(){
    return {
      title: 'Licenses',
      path: cfg.routes.hubLicenses,
      exact: true,
      component: this.Component
    }
  }

  onload({ featureFlags }) {
    if(!featureFlags.hubLicenses()){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      title: 'Licenses',
      to: cfg.routes.hubLicenses
    });
  }

}

export default LicensesFeature;