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

import withFeature, { FeatureBase } from 'oss-app/components/withFeature';
import cfg from 'e-app/config'
import HubCatalog from './../components/HubCatalog';
import { addTopNavItem } from './../flux/nav/actions';
import { fetchApps } from './../flux/catalog/actions';

class FeatureHubCatalog extends FeatureBase {

  constructor(){
    super();
    this.Component = withFeature(this)(HubCatalog);
  }

  getRoute(){
    return {
      title: 'Catalog',
      path: cfg.routes.hubCatalog,
      exact: true,
      component: this.Component
    }
  }

  onload({ featureFlags }) {
    const allowed = featureFlags.hubApps();
    if(!allowed){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      title: 'Catalog',
      to: cfg.routes.hubCatalog
    });

    this.setProcessing();
    fetchApps()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureHubCatalog;