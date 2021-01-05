import Certificate from 'oss-app/cluster/components/Certificate'
import { fetchTlsCert } from 'oss-app/cluster/flux/tlscert/actions';
import withFeature, { FeatureBase } from 'oss-app/components/withFeature';
import * as featureFlags from 'oss-app/cluster/featureFlags';
import cfg from 'e-app/config'
import { addSettingNavItem } from 'e-app/hub/flux/nav/actions';
import * as Icons from 'shared/components/Icon';

class FeatureHubCertificate extends FeatureBase {

  constructor() {
    super();
    this.Component = withFeature(this)(Certificate);
  }

  getRoute(){
    return {
      title: 'Certificate',
      path: cfg.routes.hubSettingCert,
      exact: true,
      component: this.Component
    }
  }

  onload() {
    if(!featureFlags.clusterCert()){
      this.setDisabled();
      return;
    }

    addSettingNavItem({
      title: 'HTTPS Certificate',
      Icon: Icons.License,
      exact: true,
      to: cfg.routes.hubSettingCert
    });

    this.setProcessing();
    return fetchTlsCert(cfg.defaultSiteId)
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureHubCertificate;