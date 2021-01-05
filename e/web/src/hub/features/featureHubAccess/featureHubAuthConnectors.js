import cfg from 'e-app/config'
import { addUserRoleNavItem } from 'e-app/hub/flux/nav/actions';
import FeatureAuthConnectors, { makeNavItem } from 'e-app/cluster/features/featureAuthConnectors';

class FeatureHubConnectors extends FeatureAuthConnectors {

  getRoute(){
    return {
      ...super.getRoute(),
      path: cfg.routes.hubAccessAuth
    }
  }

  onload(context) {
    super.onload(context);
    if(!this.isDisabled()){
      const item = makeNavItem(cfg.routes.hubAccessAuth);
      addUserRoleNavItem(item);
    }
  }
}

export default FeatureHubConnectors;