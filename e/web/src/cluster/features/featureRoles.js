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
import cfg from 'e-app/config';
import { fetchRoles } from 'e-app/cluster/flux/roles/actions';
import Roles from './../components/Roles';

export const makeNavItem = to => ({
  title: 'Roles',
  Icon: Icons.ClipboardUser,
  to
})

class FeatureRoles extends FeatureBase {

  constructor() {
    super()
    this.Component = withFeature(this)(Roles);
  }

  getRoute(){
    return {
      title: 'Roles',
      path: cfg.routes.clusterRoles,
      exact: true,
      component: this.Component
    }
  }

  onload(context) {
    const allowed = context.featureFlags.clusterRoles()
    if (!allowed) {
      this.setDisabled();
      return;
    }

    const navItem = makeNavItem(cfg.getClusterRolesRoute());
    addSideNavItem(navItem);

    this.setProcessing();
    fetchRoles()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureRoles;