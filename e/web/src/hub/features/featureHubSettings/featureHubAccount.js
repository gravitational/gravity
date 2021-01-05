import * as Icons from 'shared/components/Icon';
import Account from 'oss-app/cluster/components/Account';
import withFeature, { FeatureBase } from 'oss-app/components/withFeature';
import { addSettingNavItem } from 'e-app/hub/flux/nav/actions';
import cfg from 'e-app/config'

class FeatureAccount extends FeatureBase {

  constructor() {
    super()
    this.Component = withFeature(this)(Account);
  }

  getRoute(){
    return {
      title: 'Account',
      path: cfg.routes.hubSettingAccount,
      exact: true,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    if(!featureFlags.siteAccount()){
      this.setDisabled();
      return;
    }

    addSettingNavItem({
      title: 'Account Settings',
      Icon: Icons.User,
      to: cfg.routes.hubSettingAccount
    });
  }

}

export default FeatureAccount;