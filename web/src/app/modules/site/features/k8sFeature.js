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

import $ from 'jquery';
import cfg from 'app/config'
import FeatureBase from '../../featureBase'
import SiteK8sPage from './../components/k8sPage/main.jsx';
import SiteK8sNodesTab from './../components/k8sPage/nodesTab.jsx';
import SiteK8sPodsTab from './../components/k8sPage/podsTab.jsx';
import SiteK8sJobsTab from './../components/k8sPage/jobsTab.jsx';
import SiteK8sServicesTab from './../components/k8sPage/servicesTab.jsx';
import SiteK8sDaemonSetsTab from './../components/k8sPage/daemonSetsTab.jsx';
import SiteK8sDeploymentsTab from './../components/k8sPage/deploymentsTab.jsx';
import * as initActionTypes from './actionTypes';

// actions
import {addNavItem} from './../flux/currentSite/actions';
import {fetchNodes} from './../flux/k8sNodes/actions';
import {fetchServices} from './../flux/k8sServices/actions';
import {fetchNamespaces} from './../flux/k8sNamespaces/actions';
import {fetchPods} from './../flux/k8sPods/actions';
import {fetchJobs, fetchDaemonSets, fetchDeployments} from './../flux/k8s/actions';

const createRoutes = feature => ({
  path: cfg.routes.siteK8s,
  component: feature.withMe(SiteK8sPage),
  indexRoute: {
    component: SiteK8sNodesTab
  },
  childRoutes: [
    {
      path: cfg.routes.siteK8sPods,
      component: SiteK8sPodsTab
    }, {
      path: cfg.routes.siteK8sServices,
      component: SiteK8sServicesTab
    },
     {
      path: cfg.routes.siteK8sJobs,
      component: SiteK8sJobsTab
    },
    {
      path: cfg.routes.siteK8sDaemonSets,
      component: SiteK8sDaemonSetsTab
    },
    {
      path: cfg.routes.siteK8sDeployments,
      component: SiteK8sDeploymentsTab
    }
  ]
})

const navItem = siteId => ({
  icon: 'fa fa-cubes',
  to: cfg.getSiteK8sRoute(siteId),
  title: 'Kubernetes'
})

class K8sFeature extends FeatureBase {

  constructor(routes) {
    super(initActionTypes.K8S)
    routes.push(createRoutes(this));
  }

  activate() {
    try {
      this.stopProcessing()
    } catch (err) {
      this.handleError(err)
    }
  }

  onload(context) {
    if (!context.featureFlags.siteK8s()) {
      this.handleAccesDenied();
      return;
    }

    addNavItem(navItem(context.siteId))

    this.startProcessing();
    $.when(
      fetchDeployments(),
      fetchNodes(),
      fetchServices(),
      fetchNamespaces(),
      fetchPods(),
      fetchJobs(),
      fetchDaemonSets())
      .done(this.activate.bind(this))
      .fail(this.handleError.bind(this));
  }
}

export default K8sFeature;