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
import Site from './components/main.jsx';
import OffLine from './components/offline.jsx';
import SiteUninstall from './components/uninstall';
import SiteAppPage from './components/appPage/main.jsx';
import { initCluster } from './flux/actions';
import { initUninstaller } from './flux/uninstall/actions';
import FeatureActivator from './../featureActivator';
import ConfigMapsFeature from './features/configMapsFeature';
import ServersFeature from './features/serversFeature';
import OperationsFeature from './features/operationsFeature';
import LogsFeature from './features/logsFeature';
import MonitorFeature from './features/monitorFeature';
import K8sFeature from './features/k8sFeature';
import InfoFeature from './features/infoFeature';
import './flux';

const featureActivator = new FeatureActivator();
const featureRoutes = [
  {
    path: cfg.routes.siteApp,
    component: SiteAppPage
  }
]

const features = [
  new InfoFeature(featureRoutes),
  new ServersFeature(featureRoutes),
  new OperationsFeature(featureRoutes),
  new LogsFeature(featureRoutes),
  new MonitorFeature(featureRoutes),
  new K8sFeature(featureRoutes),
  new ConfigMapsFeature(featureRoutes)
]

features.forEach( f => featureActivator.register(f) );

const onEnterSite = nextState => {          
  initCluster(nextState.params.siteId, featureActivator);
}

const onEnterSiteUninstall = nextState => {          
  initUninstaller(nextState.params.siteId);
}

const routes = [    
  {
    title: 'Cluster',
    childRoutes: [  
    { path: cfg.routes.siteOffline, component: OffLine },  
    { path: cfg.routes.siteUninstall, onEnter: onEnterSiteUninstall, component: SiteUninstall },  
    {
      onEnter: onEnterSite,
      component: Site,
      indexRoute: { component: SiteAppPage },
      childRoutes: featureRoutes
    }]
  }    
];

export default routes;
