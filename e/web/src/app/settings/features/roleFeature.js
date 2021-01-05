import htmlUtils from 'oss-app/lib/htmlUtils';
import FeatureBase from 'oss-app/modules/featureBase'
import { addNavItem } from 'oss-app/modules/settings/flux/actions';
import { NavGroupEnum } from 'oss-app/modules/settings/enums';

import cfg from './../../config';
import Roles from './../components/roles/main';
import { fetchRoles } from './../flux/roles/actions';
import * as featureFlags from './featureFlags';

class FeatureRoles extends FeatureBase {

  constructor(routes) {
    super()
    routes.push({
      path: this.getIndexRoute(),
      component: super.withMe(Roles)
    });
  }

  getIndexRoute(){
    return cfg.routes.settingsRoles;
  }

  onload(context) {
    const allowed = featureFlags.settingsRole()
    if (!allowed) {
      this.handleAccesDenied();
      return;
    }

    const navItem = {
      icon: 'fa fa-user-secret',
      title: 'Roles',
      to: htmlUtils.joinPaths(context.baseUrl, this.getIndexRoute())
    }

    addNavItem(NavGroupEnum.USER_GROUPS, navItem);
    this.startProcessing();
    fetchRoles()
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this));
  }
}

export default FeatureRoles