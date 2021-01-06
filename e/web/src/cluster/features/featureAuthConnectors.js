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
import { addSideNavItem } from 'oss-app/cluster/flux/nav/actions';
import * as Icons from 'shared/components/Icon';
import AuthConnectors from 'e-app/cluster/components/AuthConnectors';
import { fetchAuthProviders } from 'e-app/cluster/flux/authConnectors/actions';
import cfg from 'e-app/config';

export function makeNavItem(to) {
  return {
    title: 'Auth Connectors',
    Icon: Icons.Lock,
    to
  }
}

class FeatureAuthConnectors extends FeatureBase {

  constructor() {
    super()
    this.Component = withFeature(this)(AuthConnectors);
  }

  getRoute(){
    return {
      title: 'Auth. Connectors',
      path: cfg.routes.clusterAuthConnectors,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    const allowed = featureFlags.clusterAuthConnectors();
    if (!allowed) {
      this.setDisabled();
      return;
    }

    const navItem = makeNavItem(cfg.getClusterAuthConnectorsRoute());
    addSideNavItem(navItem);

    this.setProcessing();
    fetchAuthProviders()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureAuthConnectors;