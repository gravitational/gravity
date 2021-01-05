/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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