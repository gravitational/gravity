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

import cfg from 'app/config'
import LogForwarders from './../components/logForwarders/main';
import FeatureBase from './../../featureBase'
import * as featureFlags from './../../featureFlags';
import { fetchForwarders } from './../flux/logForwarders/actions';
import htmlUtils from 'app/lib/htmlUtils';
import { addNavItem } from './../flux/actions';
import { NavGroupEnum } from './../enums';

class LogFeature extends FeatureBase {

  context = null

  constructor(routes) {
    super()
    routes.push({
      path: this.getIndexRoute(),
      component: super.withMe(LogForwarders)
    });
  }

  getIndexRoute(){
    return cfg.routes.settingsMetricsLogs;
  }

  componentDidMount(){
    this.init();
  }

  init() {
    if (!this.wasInitialized()) {
      this.startProcessing();
      fetchForwarders(this.context.siteId)
        .done(this.stopProcessing.bind(this))
        .fail(this.handleError.bind(this));
    }
  }

  onload(context) {
    this.context = context;
    const enabled = featureFlags.settingsLogForwarder();
    if(enabled) {
      const navItem = {
        icon: 'fa fa-book',
        title: 'Logs',
        to: htmlUtils.joinPaths(context.baseUrl, this.getIndexRoute())
      }
      addNavItem(NavGroupEnum.SETTINGS, navItem);
      this.init();
    }
  }
}

export default LogFeature;