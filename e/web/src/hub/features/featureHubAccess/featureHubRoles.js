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

import cfg from 'e-app/config'
import { addUserRoleNavItem } from 'e-app/hub/flux/nav/actions';
import FeatureRoles, { makeNavItem } from 'e-app/cluster/features/featureRoles';

class FeatureHubRoles extends FeatureRoles {

  getRoute(){
    return {
      ...super.getRoute(),
      path: cfg.routes.hubAccessRoles
    }
  }

  onload(context) {
    super.onload(context);
    if(!this.isDisabled()){
      const item = makeNavItem(cfg.routes.hubAccessRoles);
      addUserRoleNavItem(item);
    }
  }
}

export default FeatureHubRoles;