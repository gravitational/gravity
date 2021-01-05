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

import * as Icons from 'shared/components/Icon';
import Account from 'oss-app/cluster/components/Account';
import withFeature, { FeatureBase } from 'oss-app/components/withFeature';
import { addSettingNavItem } from 'e-app/hub/flux/nav/actions';
import cfg from 'e-app/config'

class FeatureAccount extends FeatureBase {

  constructor() {
    super()
    this.Component = withFeature(this)(Account);
  }

  getRoute(){
    return {
      title: 'Account',
      path: cfg.routes.hubSettingAccount,
      exact: true,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    if(!featureFlags.siteAccount()){
      this.setDisabled();
      return;
    }

    addSettingNavItem({
      title: 'Account Settings',
      Icon: Icons.User,
      to: cfg.routes.hubSettingAccount
    });
  }

}

export default FeatureAccount;