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