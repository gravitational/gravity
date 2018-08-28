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

import cfg from 'app/config';
import MetricsMonitor from './../components/metrics/main.jsx';
import { fetchRetentionValues } from './../flux/metrics/actions';
import FeatureBase from './../../featureBase'
import * as featureFlags from './../../featureFlags';
import htmlUtils from 'app/lib/htmlUtils';
import { addNavItem } from './../flux/actions';
import { NavGroupEnum } from './../enums';

class MonitorFeature extends FeatureBase {
  constructor(routes) {
    super()
    routes.push({
      path: this.getIndexRoute(),
      component: super.withMe(MetricsMonitor)
    });
  }

  getIndexRoute(){
    return cfg.routes.settingsMetricsMonitor;
  }

  onload(context) {
    const { siteId } = context;
    const isEnabled = featureFlags.settingsMonitoring();
    const navItem = {
      icon: 'fa fa-area-chart',
      title: 'Monitoring',
      to: htmlUtils.joinPaths(context.baseUrl, this.getIndexRoute())
    }

    if (!isEnabled) {
      this.handleAccesDenied();
      return;
    }

    addNavItem(NavGroupEnum.SETTINGS, navItem);
    this.startProcessing();
    fetchRetentionValues(siteId)
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this));
  }
}

export default MonitorFeature;