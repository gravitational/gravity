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
import Settings from './components/main'
import SettingsIndex from './components/index'
import { initSettings } from './flux/actions';
import FeatureActivator from './../featureActivator';
import CertFeature from './features/certFeature';
import UserFeature from './features/userFeature';
import LogForwarderFeature from './features/logForwarderFeature';
import MonitorFeature from './features/monitorFeature';
import AccountFeature from './features/accountFeature';
import './flux';

const featureRoutes = []
const featureActivator = new FeatureActivator();
featureActivator.register(new AccountFeature(featureRoutes));
featureActivator.register(new UserFeature(featureRoutes));
featureActivator.register(new LogForwarderFeature(featureRoutes));
featureActivator.register(new MonitorFeature(featureRoutes));
featureActivator.register(new CertFeature(featureRoutes));

const onEnter = (nextState) => {
  const { siteId } = nextState.params;
  const baseLabel = cfg.getSettingsClusterHeaderText();
  const goBackUrl = cfg.getSiteRoute(siteId);
  const baseUrl = cfg.getSiteSettingsRoute(siteId);

  const activationContext = {
    baseLabel,
    goBackUrl,
    siteId,
    baseUrl
  };

  initSettings(activationContext, featureActivator);
}

const routes = {
  title: 'Settings',
  onEnter: onEnter,
  component: Settings,
  indexRoute: {
    // need index component to handle default route
    component: SettingsIndex
  },
  childRoutes: featureRoutes
}

export default [routes];