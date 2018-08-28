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

import { Router, createMemoryHistory } from 'react-router';
import { Provider } from 'nuclear-js-react-addons';
import reactor from 'app/reactor';
import expect from 'expect';
import cfg from 'app/config';
import api from 'app/services/api';
import { $, spyOn } from 'app/__tests__/';
import { makeHelper } from 'app/__tests__/domUtils';
import React from 'react';
import * as ajaxUtils  from 'app/__tests__/ajaxUtils';
import * as ReactDOM from 'react-dom';
import * as fakeData from 'app/__tests__/apiData'
import clusterRoutes from './../index';
import data from './data.json';

const routes = {
 path: '/web/site/:siteId',
 childRoutes: clusterRoutes
}

const $node = $('<div>');
const helper = makeHelper($node);
const withDisabledFeatures = siteJson => ({
  ...siteJson,
  app: {
    ...siteJson.app,
    manifest: {
      extensions: {
        monitoring: {
          disabled: true
        },
        kubernetes: {
          disabled: true
        },
        configuration: {
          disabled: true
        },
        logs: {
          disabled: true
        }
      }
    }
  }
})

describe('app/modules/site/main', () => {
  let fapi = null;

  const portal = prefix => `/portalapi/v1/sites/samplecluster${prefix}`;
  const k8s = prefix => `/sites/v1/fakeAccount/samplecluster/proxy/master/k8s/api/v1${prefix}`;
  const emptyK8sResponse = { items: [] };
  beforeEach(() => {
    helper.setup();
    setAcl({});

    fapi = ajaxUtils.mock(api)
    fapi.get('/portalapi/v1/sites/samplecluster/servers').andResolve([])
    fapi.get('/proxy/v1/webapi/sites/samplecluster/nodes').andResolve({ nodes: [] });
    fapi.get(portal('/apps')).andResolve(null)
    fapi.get(portal('/operations')).andResolve([])
    fapi.get(portal('/access')).andResolve(fakeData.siteAccessResp)
    fapi.get(portal('/endpoints')).andResolve(null)
    fapi.get(portal('')).andResolve(fakeData.siteResp)

    fapi.get(k8s('/pods')).andResolve(emptyK8sResponse);
    fapi.get(k8s('/pods')).andResolve(emptyK8sResponse);
    fapi.get('/sites/v1/fakeAccount/samplecluster/proxy/master/k8s/apis/batch/v1/jobs').andResolve(emptyK8sResponse);
    fapi.get('/sites/v1/fakeAccount/samplecluster/proxy/master/k8s/apis/extensions/v1beta1/deployments').andResolve(emptyK8sResponse);
    fapi.get('/sites/v1/fakeAccount/samplecluster/proxy/master/k8s/apis/extensions/v1beta1/daemonsets').andResolve(emptyK8sResponse);
    fapi.get(k8s('/pods')).andResolve(emptyK8sResponse);
    fapi.get(k8s('/namespaces/default/pods')).andResolve(emptyK8sResponse);
    fapi.get(k8s('/configmaps')).andResolve(emptyK8sResponse);
    fapi.get(k8s('/namespaces')).andResolve(emptyK8sResponse);
    fapi.get(k8s('/nodes')).andResolve(emptyK8sResponse);
    fapi.get(k8s('/services')).andResolve(emptyK8sResponse);
    spyOn(cfg, 'init');
    spyOn(cfg, 'isRemoteAccess').andReturn(false);
    spyOn(cfg, 'isDevCluster').andReturn(false);
  })

  afterEach(function () {
    helper.clean();
    reactor.reset();
    expect.restoreSpies();
  })

  describe('admin (summary) feature', function () {
    const history = new createMemoryHistory();
    it('should render', () => {
      history.push('/web/site/samplecluster')
      render(history);
      expectActiveNavItem('Admin');
      helper.shouldExist('.grv-site-app-cur-ver')
    });
  });

  describe('servers feature', function () {
    const history = new createMemoryHistory();
    it('should render', () => {
      history.push('/web/site/samplecluster/servers')
      render(history);
      expectActiveNavItem('Servers');
      helper.shouldExist('.grv-site-servers-provisioner-header');
    });
  });

  describe('history feature', function () {
    const history = new createMemoryHistory();
    it('should render', () => {
      history.push('/web/site/samplecluster/history')
      render(history);
      expectActiveNavItem('Operations');
      helper.shouldExist('.grv-site-history');
    });
  });

  describe('disable features', function () {
    const history = new createMemoryHistory();
    it('from manifest', () => {
      fapi.get('/portalapi/v1/sites/samplecluster').andResolve(withDisabledFeatures(fakeData.siteResp))
      history.push('/web/site/samplecluster')
      render(history);
      helper.shouldNotExist('a[href$="logs"]');
      helper.shouldNotExist('a[href$="monitor"]');
      helper.shouldNotExist('a[href$="cfg"]');
      helper.shouldNotExist('a[href$="k8s"]');
    });

    it('from user ACL ', () => {
      setAcl({ clusters: { connect: false } });
      history.push('/web/site/samplecluster')
      render(history);
      helper.shouldNotExist('a[href$="monitor"]');
      helper.shouldNotExist('a[href$="k8s"]');
      helper.shouldNotExist('a[href$="cfg"]');
    });
  })

  describe('logs feature', function () {
    const history = new createMemoryHistory();
    it('should render', () => {
      const logsFailed = $.Deferred().reject(new Error());
      logsFailed.abort = () => { };
      fapi.get('/sites/v1/fakeAccount/samplecluster/proxy/master/logs/log?query=').andReturnPromise(logsFailed);
      history.push('/web/site/samplecluster/logs')
      render(history);
      expectActiveNavItem('Logs');
      helper.shouldExist('.grv-logviewer-body');
    });
  });

  describe('monitor feature', function () {
    const history = new createMemoryHistory();
    it('should render', () => {
      fapi.put('/portalapi/v1/sites/samplecluster/grafana').andReject(new Error());
      history.push('/web/site/samplecluster/monitor')
      render(history);
      expectActiveNavItem('Monitoring');
      helper.shouldExist('.grv-site-monitor');
    });
  });

  describe('k8s feature', function () {
    const history = new createMemoryHistory();
    it('should render', () => {
      history.push('/web/site/samplecluster/k8s')
      render(history);
      expectActiveNavItem('Kubernetes');
      helper.shouldExist('.grv-site-k8s');
    });
  });

  describe('cfg feature', function () {
    const history = new createMemoryHistory();
    it('should render', () => {
      history.push('/web/site/samplecluster/cfg')
      render(history);
      expectActiveNavItem('Configuration');
      helper.shouldExist('.grv-site-configs');
    });
  });
});

const setAcl = acl => {
  acl = {
    ... data.userAcl,
    ...acl
  }

  reactor.dispatch('USER_RECEIVE_DATA', data.user);
  reactor.dispatch('USERACL_RECEIVE', acl);
}

const render = history => {
  ReactDOM.render((
  <Provider reactor={reactor}>
    <Router history={history} routes={routes} />
  </Provider>),
  $node[0]);
}

function expectActiveNavItem(expected) {
  const text = $node
    .find('.navbar-static-side .active')
    .text();
  expect(text).toMatch(expected)
}