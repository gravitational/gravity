import FeatureBase from 'oss-app/modules/featureBase'
import htmlUtils from 'oss-app/lib/htmlUtils';
import { addNavItem } from 'oss-app/modules/settings/flux/actions';
import { NavGroupEnum } from 'oss-app/modules/settings/enums';

import * as featureFlags from './featureFlags';
import Auth from './../components/auth/main';
import cfg from './../../config';
import {fetchAuthProviders} from './../flux/auth/actions';

class AuthConnectorsFeature extends FeatureBase {

  constructor(routes) {
    super()
    routes.push({
      path: this.getIndexRoute(),
      component: super.withMe(Auth)
    });
  }

  getIndexRoute(){
    return cfg.routes.settingsAuth;
  }

  onload(context) {
    const allowed = featureFlags.settingsAuth();
    const navItem = {
      icon: 'fa fa-connectdevelop',
      title: 'Auth Connectors',
      to: htmlUtils.joinPaths(context.baseUrl, this.getIndexRoute())
    }

    if (!allowed) {
      this.handleAccesDenied();
      return;
    }

    addNavItem(NavGroupEnum.USER_GROUPS, navItem);
    this.startProcessing();
    fetchAuthProviders(context.siteId)
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this));
  }
}

export default AuthConnectorsFeature;