/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import * as ReactDOM from 'react-dom';
import { Router, createMemoryHistory } from 'react-router';
import { Provider } from 'nuclear-js-react-addons';
import expect from 'expect';

import reactor from 'oss-app/reactor';
import cfg from 'oss-app/config';
import api from 'oss-app/services/api';
import { $, spyOn } from 'oss-app/__tests__/';
import * as ajaxUtils  from 'oss-app/__tests__/ajaxUtils';
import * as fakeData from 'oss-app/__tests__/apiData';

import settingsRoutes from './../cluster';
import webcontext from './data.json';

const $node = $('<div>').appendTo("body");

describe('enterprise/settings/cluster', () => {

  let fapi = null;

  beforeEach(() => {
    fapi = ajaxUtils.mock(api)
    fapi.get('/portalapi/v1/sites/samplecluster/certificate').andResolve(fakeData.certificate)
    fapi.get('/portalapi/v1/sites/samplecluster').andResolve(fakeData.siteResp)
    fapi.get('/portalapi/v1/sites/samplecluster/monitoring/retention').andResolve(null)
    fapi.get('/portalapi/v1/sites/samplecluster/resources/logforwarder').andResolve([])
    fapi.get('/portalapi/v1/sites/samplecluster/resources/auth_connector').andResolve([])
    fapi.get('/portalapi/v1/sites/samplecluster/resources/role').andResolve([])
    fapi.get('/portalapi/v1/accounts/existing/users').andResolve(null)
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
      render(routes, history);
      expectActiveNavItem('My Account');
      expectNavItemUrl('/web/site/samplecluster/settings/account');
      expectNavItemUrl('/web/site/samplecluster/settings/roles');
      expectNavItemUrl('/web/site/samplecluster/settings/auth');
      expectNavItemUrl('/web/site/samplecluster/settings/users');
      expectNavItemUrl('/web/site/samplecluster/settings/logs');
      expectNavItemUrl('/web/site/samplecluster/settings/monitoring');
      expectNavItemUrl('/web/site/samplecluster/settings/cert');
    })
  })

  describe('auth feature', function () {
    const history = new createMemoryHistory();
    it('can be enabled from acl', () => {
      const acl = {
        authConnectors: {
          list: true
        }
      }

      setAcl(acl)
      history.push('/web/site/samplecluster/settings/auth')
      render(routes, history);
      expectActiveNavItem('Auth');
    });

    it('can be disabled from acl', () => {
      const history = new createMemoryHistory();
      const acl = {
        authConnectors: {
          list: false
        }
      }

      setAcl(acl)
      history.push('/web/site/samplecluster/settings/auth')
      render(routes, history);
      expectNotActiveNavItem('Auth');
    });
  });

  describe('roles feature', function () {
    const history = new createMemoryHistory();
    it('can be enabled from acl', () => {
      const acl = {
        roles: {
          list: true
        }
      }
      setAcl(acl)
      history.push('/web/site/samplecluster/settings/roles')
      render(routes, history);
      expectActiveNavItem('Roles');
    });

    it('can be disabled from acl', () => {
      const acl = {
        roles: {
          list: false
        }
      }
      setAcl(acl)
      history.push('/web/site/samplecluster/settings/roles')
      render(routes, history);
      expectNotActiveNavItem('Roles');
    });
  });
});

const routes = {
 path: '/web/site/:siteId/settings',
 childRoutes: settingsRoutes
}

const setAcl = acl => {
  acl = acl || {};
  reactor.dispatch('USER_RECEIVE_DATA', webcontext.user);
  reactor.dispatch('USERACL_RECEIVE', acl);
}

const expectActiveNavItem = expected => {
  const text = $node.find('.grv-settings-nav-group-menu-item.active').text();
  expect(text).toMatch(expected)
}

const expectNavItemUrl = url => {
  const isFound = !!$node[0].querySelector(`a[href='${url}']`);
  expect(isFound).toBeTruthy(`cannot find nav item with: ${url}`);
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