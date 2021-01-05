import cfg from 'e-app/config'
import { addUserRoleNavItem } from 'e-app/hub/flux/nav/actions';
import FeatureUsers, { makeNavItem } from 'e-app/cluster/features/featureUsers';

class FeatureHubRoles extends FeatureUsers {

  getRoute(){
    return {
      ...super.getRoute(),
      path: cfg.routes.hubAccessUsers
    }
  }

  onload(context) {
    super.onload(context);
    if(!this.isDisabled()){
      const item = makeNavItem(cfg.routes.hubAccessUsers);
      addUserRoleNavItem(item);
    }
  }
}

export default FeatureHubRoles;