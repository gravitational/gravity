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
import FeatureBase from '../../featureBase'
import MonitorPage from './../components/monitor/main.jsx';
import {addNavItem} from './../flux/currentSite/actions';

const routeWithFeature = feature => ({
  path: cfg.routes.siteMonitor + '*',
  component: feature.withMe(MonitorPage)
})

const navItem = siteId => ({
  icon: 'fa fa-area-chart',
  to: cfg.getSiteMonitorRoute(siteId),
  title: 'Monitoring',
  isIndex: true

})

class MonitorFeature extends FeatureBase {
  constructor(routes) {
    super()
    routes.push(routeWithFeature(this));
  }

  onload(context) {
    if (context.featureFlags.siteMonitoring()) {
      addNavItem(navItem(context.siteId));
    }else{
      this.handleAccesDenied();
    }
  }
}

export default MonitorFeature;