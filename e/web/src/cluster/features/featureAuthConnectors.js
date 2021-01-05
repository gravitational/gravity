import withFeature, { FeatureBase } from 'oss-app/components/withFeature';
import { addSideNavItem } from 'oss-app/cluster/flux/nav/actions';
import * as Icons from 'shared/components/Icon';
import AuthConnectors from 'e-app/cluster/components/AuthConnectors';
import { fetchAuthProviders } from 'e-app/cluster/flux/authConnectors/actions';
import cfg from 'e-app/config';

export function makeNavItem(to) {
  return {
    title: 'Auth Connectors',
    Icon: Icons.Lock,
    to
  }
}

class FeatureAuthConnectors extends FeatureBase {

  constructor() {
    super()
    this.Component = withFeature(this)(AuthConnectors);
  }

  getRoute(){
    return {
      title: 'Auth. Connectors',
      path: cfg.routes.clusterAuthConnectors,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    const allowed = featureFlags.clusterAuthConnectors();
    if (!allowed) {
      this.setDisabled();
      return;
    }

    const navItem = makeNavItem(cfg.getClusterAuthConnectorsRoute());
    addSideNavItem(navItem);

    this.setProcessing();
    fetchAuthProviders()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureAuthConnectors;