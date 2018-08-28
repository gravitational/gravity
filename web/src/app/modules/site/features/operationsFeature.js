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
import SiteHistoryPage from './../components/historyPage/main.jsx';
import {addNavItem} from './../flux/currentSite/actions';

const route = {
  path: cfg.routes.siteHistory,
  component: SiteHistoryPage
}

const navItem = siteId => ({
  icon: 'fa fa-history',
  to: cfg.getSiteHistoryRoute(siteId),
  title: 'Operations'
})

class OperationFeature extends FeatureBase {
  constructor(routes) {
    super()
    routes.push(route);
  }

  onload(context) {
    addNavItem(navItem(context.siteId))
  }
}

export default OperationFeature;