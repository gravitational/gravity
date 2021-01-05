import cfg from 'e-app/config';
import withFeature, { FeatureBase, Activator } from 'oss-app/components/withFeature';
import { addTopNavItem } from 'e-app/hub/flux/nav/actions';
import HubAccess from 'e-app/hub/components/HubAccess';
import FeatureRoles from './featureHubRoles';
import FeatureUsers from './featureHubUsers';
import FeatureAuthConn from './featureHubAuthConnectors';

class FeatureHubAccess extends FeatureBase {

  // index route
  path = cfg.routes.hubAccess

  constructor() {
    super();
    this.features = [
      new FeatureUsers(),
      new FeatureRoles(),
      new FeatureAuthConn()
    ]

    this.Component = withFeature(this)(HubAccess);
  }

  getRoute(){
    return {
      title: 'User/Auth',
      path: this.path,
      exact: false,
      component: this.Component
    }
  }

  onload(context) {
    const activator = new Activator(this.features);
    activator.onload(context);

    const isDisabled = this.features.every( f => f.isDisabled() );
    if(isDisabled){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      exact: false,
      title: 'User/Auth',
      to: this.path
    })
  }
}

export default FeatureHubAccess;