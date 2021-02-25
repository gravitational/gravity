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

import { addTopNavItem } from 'oss-app/cluster/flux/nav/actions';
import withFeature, { FeatureBase } from 'oss-app/components/withFeature';
import * as Icons from 'shared/components/Icon';
import cfg from 'e-app/config'
import License from './../components/License';

class LicenseFeature extends FeatureBase {
  constructor() {
    super()
    this.Component = withFeature(this)(License);
  }

  getRoute(){
    return {
      title: 'License',
      path: cfg.routes.siteLicense,
      exact: true,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    if(!featureFlags.clusterLicense()){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      title: 'License',
      Icon: Icons.License,
      to: cfg.getSiteLicenseRoute()
    });
  }
}

export default LicenseFeature