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

import cfg from 'e-app/config';
import withFeature, { FeatureBase, Activator } from 'oss-app/components/withFeature';
import { addTopNavItem } from 'e-app/hub/flux/nav/actions';
import HubAccess from 'e-app/hub/components/HubAccess';
import FeatureRoles from './featureHubRoles';
import FeatureUsers from './featureHubUsers';
import FeatureAuthConn from './featureHubAuthConnectors';

class FeatureHubAccess extends FeatureBase {

  // index route
  path = cfg.routes.hubAccess

  constructor() {
    super();
    this.features = [
      new FeatureUsers(),
      new FeatureRoles(),
      new FeatureAuthConn()
    ]

    this.Component = withFeature(this)(HubAccess);
  }

  getRoute(){
    return {
      title: 'User/Auth',
      path: this.path,
      exact: false,
      component: this.Component
    }
  }

  onload(context) {
    const activator = new Activator(this.features);
    activator.onload(context);

    const isDisabled = this.features.every( f => f.isDisabled() );
    if(isDisabled){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      exact: false,
      title: 'User/Auth',
      to: this.path
    })
  }
}

export default FeatureHubAccess;