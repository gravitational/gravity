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
import React from 'react';
import settingsRoutes from './../index';

import * as ReactDOM from 'react-dom';
import * as ajaxUtils  from 'app/__tests__/ajaxUtils';
import * as fakeData from 'app/__tests__/apiData';

const $node = $('<div>').appendTo("body");

import webcontext from './data.json';

describe('app/modules/settings/main', () => {

  let fapi = null;

  beforeEach(() => {
    fapi = ajaxUtils.mock(api)
    fapi.get('/portalapi/v1/sites/samplecluster/certificate').andResolve(fakeData.certificate)
    fapi.get('/portalapi/v1/sites/samplecluster').andResolve(fakeData.siteResp)
    fapi.get('/portalapi/v1/sites/samplecluster/monitoring/retention').andResolve(null)
    fapi.get('/portalapi/v1/sites/samplecluster/resources/logforwarder').andResolve([])
    fapi.get('/portalapi/v1/sites/samplecluster/resources/roles').andResolve(null)
    fapi.get('/portalapi/v1/accounts/existing/users').andResolve(null)
    spyOn(cfg, 'init');
    spyOn(cfg, 'isRemoteAccess').andReturn(false);
    spyOn(cfg, 'isDevCluster').andReturn(false);
  })

  afterEach(function () {
    clean($node);
  })

  describe('when hitting index route', function () {
    const history = new createMemoryHistory();
    it('should redirect to first available', () => {
      setAcl(webcontext.userAcl)
      history.push('/web/site/samplecluster/settings')
      render(clusterRoutes, history);
      expectActiveNavItem('My Account');
      expectNavItemUrl('/web/site/samplecluster/settings/account');
      expectNavItemUrl('/web/site/samplecluster/settings/users');
      expectNavItemUrl('/web/site/samplecluster/settings/logs');
      expectNavItemUrl('/web/site/samplecluster/settings/monitoring');
      expectNavItemUrl('/web/site/samplecluster/settings/cert');
    })
  })

  describe('monitoring feature', function () {
    const history = new createMemoryHistory();
    it('should render', () => {
      spyOn(cfg, 'isSettingsMonitoringEnabled').andReturn(true)
      history.push('/web/site/samplecluster/settings/monitoring')
      render(clusterRoutes, history);
      expectActiveNavItem('Monitoring');
    });

    it('can be disabled from cfg', () => {
      setAcl()
      spyOn(cfg, 'isSettingsMonitoringEnabled').andReturn(false)
      history.push('/web/site/samplecluster/settings/monitoring')
      render(clusterRoutes, history);
      expectNotActiveNavItem('Monitoring');
    });
  });

  describe('certificate feature', function () {
    const history = new createMemoryHistory();
    it('should render', () => {
      history.push('/web/site/samplecluster/settings/cert')
      render(clusterRoutes, history);
      expectActiveNavItem('HTTPS Certificate');
    });
  });

  describe('users feature', function () {
    const history = new createMemoryHistory();
    it('can be enabled by acl', () => {
      const acl = {
        users: {
          list: true
        }
      }
      setAcl(acl)
      history.push('/web/site/samplecluster/settings/users')
      render(clusterRoutes, history);
      expectActiveNavItem('Users');
    });

    it('can be disabled from acl', () => {
      const acl = {
        users: {
          list: false
        }
      }
      setAcl(acl)
      history.push('/web/site/samplecluster/settings/users')
      render(clusterRoutes, history);
      expectNotActiveNavItem('Users');
    });
  });

  describe('logForwarders feature', function () {
    const history = new createMemoryHistory();
    it('can be disabled from acl', () => {
      const acl = { logForwarders: { list : false } };
      setAcl(acl)
      history.push('/web/site/samplecluster/settings/logs')
      render(clusterRoutes, history);
      expectNotActiveNavItem('Logs');
    });

    it('can be disabled from cfg', () => {
      setAcl({})
      spyOn(cfg, 'isSettingsLogsEnabled').andReturn(false);
      history.push('/web/site/samplecluster/settings/logs')
      render(clusterRoutes, history);
      expectNotActiveNavItem('Logs');
    });
  });
});

const clusterRoutes = {
 path: '/web/site/:siteId/settings',
 childRoutes: settingsRoutes
}

const setAcl = acl => {
  acl = acl || {}
  reactor.dispatch('USER_RECEIVE_DATA', webcontext.user);
  reactor.dispatch('USERACL_RECEIVE', acl);
}

const expectActiveNavItem = expected => {
  const text = $node.find('.grv-settings-nav-group-menu-item.active').text();
  expect(text).toMatch(expected)
}

const expectNavItemUrl = url => {
  const isFound = !!$node[0].querySelector(`a[href='${url}']`);
  expect(isFound).toBeTruthy(`cannot fine nav item with: ${url}`);
}

const expectNotActiveNavItem = expected => {
  const text = $node.find('.grv-settings-nav-group-menu-item.active').text();
  expect(text).toNotMatch(expected)
}

const render = (routes, history) => {
  ReactDOM.render((
  <Provider reactor={reactor}>
    <Router history={history} routes={routes} />
  </Provider>),
  $node[0]);
}

function clean(el){
  if (el) {
    ReactDOM.unmountComponentAtNode($(el)[0]);
  }

  expect.restoreSpies();
}