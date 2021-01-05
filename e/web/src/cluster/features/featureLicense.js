import { addTopNavItem } from 'oss-app/cluster/flux/nav/actions';
import withFeature, { FeatureBase } from 'oss-app/components/withFeature';
import * as Icons from 'shared/components/Icon';
import cfg from 'e-app/config'
import License from './../components/License';

class LicenseFeature extends FeatureBase {
  constructor() {
    super()
    this.Component = withFeature(this)(License);
  }

  getRoute(){
    return {
      title: 'License',
      path: cfg.routes.siteLicense,
      exact: true,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    if(!featureFlags.clusterLicense()){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      title: 'License',
      Icon: Icons.License,
      to: cfg.getSiteLicenseRoute()
    });
  }
}

export default LicenseFeature