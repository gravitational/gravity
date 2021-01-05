// oss imports
import FeatureBase from 'oss-app/modules/featureBase'
import htmlUtils from 'oss-app/lib/htmlUtils';
import { addNavItem } from 'oss-app/modules/settings/flux/actions';
import { NavGroupEnum } from 'oss-app/modules/settings/enums';

// local imports
import LicenseGenerator from './../components/licenseGen/licenseGenerator';
import * as featureFlags from './featureFlags';
import { initLicenseGen } from './../flux/license/actions';
import cfg from './../../config';

class LicenseFeature extends FeatureBase {

  constructor(routes) {
    super()
    routes.push({
      path: this.getIndexRoute(),
      onEnter: initLicenseGen,
      component: super.withMe(LicenseGenerator)
    });
  }

  getIndexRoute(){
    return cfg.routes.settingsLicense;
  }

  onload(context) {
    const { baseUrl } = context;
    const enabled = featureFlags.settingsLicense();
    const navItem = {
      icon: 'fa fa-key',
      title: 'Licenses',
      to: htmlUtils.joinPaths(baseUrl, this.getIndexRoute())
    }

    if (enabled) {
      addNavItem(NavGroupEnum.APPLICATION, navItem);
    }
  }
}

export default LicenseFeature;