/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import $ from 'jQuery';
import cfg from 'app/config'
import FeatureBase from '../../featureBase'
import * as initActionTypes from './actionTypes';
import {addNavItem} from './../flux/currentSite/actions';
import SiteConfigPage from './../components/configPage/main.jsx';
import {fetchNamespaces} from './../flux/k8sNamespaces/actions';
import {fetchCfgMaps, initCfgMaps} from './../flux/k8sConfigMaps/actions';

const makeNavItem = siteId => ({
  icon: 'fa fa-cogs',
  to: cfg.getSiteConfigurationRoute(siteId),
  title: 'Configuration'
})

class ConfigMapsFeature extends FeatureBase {

  constructor(routes) {
    super(initActionTypes.CFG_MAPS)
    routes.push({
      path: cfg.routes.siteConfiguration,    
      component: super.withMe(SiteConfigPage)
    })
  }

  activate() {
    try {
      initCfgMaps();
      this.stopProcessing()
    } catch (err) {
      this.handleError(err)
    }
  }

  onload(context) {
    const allowed = context.featureFlags.siteConfigMaps();
    if (!allowed) {
      this.handleAccesDenied();
      return;
    }

    addNavItem(makeNavItem(context.siteId));

    this.startProcessing();
    $.when(fetchNamespaces(), fetchCfgMaps())
      .done(this.activate.bind(this))
      .fail(this.handleError.bind(this));
  }
}

export default ConfigMapsFeature;