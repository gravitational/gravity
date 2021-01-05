import withFeature, { FeatureBase } from 'oss-app/components/withFeature';
import cfg from 'e-app/config'
import Licenses from './../components/HubLicenses';
import { addTopNavItem } from './../flux/nav/actions';

class LicensesFeature extends FeatureBase {

  constructor(){
    super();
    this.Component = withFeature(this)(Licenses);
  }

  getRoute(){
    return {
      title: 'Licenses',
      path: cfg.routes.hubLicenses,
      exact: true,
      component: this.Component
    }
  }

  onload({ featureFlags }) {
    if(!featureFlags.hubLicenses()){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      title: 'Licenses',
      to: cfg.routes.hubLicenses
    });
  }

}

export default LicensesFeature;