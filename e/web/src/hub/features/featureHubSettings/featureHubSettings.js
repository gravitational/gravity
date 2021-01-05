import cfg from 'e-app/config'
import withFeature, { FeatureBase, Activator } from 'oss-app/components/withFeature';
import { addTopNavItem } from 'e-app/hub/flux/nav/actions';
import HubSettings from 'e-app/hub/components/HubSettings';
import FeatureHubCert from './featureHubCert';
import FeatureAccount from './featureHubAccount';

class FeatureHubSettings extends FeatureBase {

  // index route to handle redirects to available features
  path = cfg.routes.hubSettings

  constructor() {
    super();
    this.features = [
      new FeatureAccount(),
      new FeatureHubCert(),
    ]

    this.Component = withFeature(this)(HubSettings);
  }

  getRoute(){
    return {
      title: 'Settings',
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
      title: 'Settings',
      to: this.path
    })
  }
}

export default FeatureHubSettings;