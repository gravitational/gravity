/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import cfg from 'app/config'
import FeatureBase from 'app/modules/featureBase';
import * as featureFlags from 'app/modules/featureFlags';
import htmlUtils from 'app/lib/htmlUtils';
import Users from './../components/users/main';
import { addNavItem } from './../flux/actions';
import { NavGroupEnum } from './../enums';
import { fetchUsers } from './../flux/users/actions';

class UsersFeature extends FeatureBase {

  constructor(routes, Component) {
    Component = Component || Users;
    super()
    routes.push({
      path: this.getIndexRoute(),
      component: super.withMe(Component)
    });
  }

  getIndexRoute(){
    return cfg.routes.settingsUsers;
  }

  onload(context) {
    const allowed = featureFlags.settingsUsers();
    if (!allowed) {
      this.handleAccesDenied();
      return;
    }

    const navItem = {
      icon: 'fa fa-users',
      title: 'Users',
      to: htmlUtils.joinPaths(context.baseUrl, this.getIndexRoute())
    }

    addNavItem(NavGroupEnum.USER_GROUPS, navItem);
    this.startProcessing();
    fetchUsers()
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this));
  }
}

export default UsersFeature

