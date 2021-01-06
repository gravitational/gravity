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
// oss imports
import reactor from 'oss-app/reactor';
import cfg from 'oss-app/config';
import api from 'oss-app/services/api';
import { $, spyOn } from 'oss-app/__tests__/';
import * as ajaxUtils  from 'oss-app/__tests__/ajaxUtils';
import * as fakeData from 'oss-app/__tests__/apiData';

// local imports
import settingsRoutes from './../portal';
import webcontext from './data.json';

const $node = $('<div>').appendTo("body");

describe('enterprise/settings/portal', () => {

  let fapi = null;

  beforeEach(() => {
    fapi = ajaxUtils.mock(api)
    fapi.get('/portalapi/v1/sites/___dev__cluster__/certificate').andResolve(fakeData.certificate)
    fapi.get('/portalapi/v1/sites/___dev__cluster__/resources/auth_connector').andResolve([]);
    fapi.get('/portalapi/v1/sites/___dev__cluster__/resources/role').andResolve([]);
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
      history.push('/web/portal/settings')
      render(routes, history);
      expectActiveNavItem('My Account');
      expectNavItemUrl('/web/portal/settings/account');
      expectNavItemUrl('/web/portal/settings/roles');
      expectNavItemUrl('/web/portal/settings/auth');
      expectNavItemUrl('/web/portal/settings/users');
      expectNavItemUrl('/web/portal/settings/license');
      expectNavItemUrl('/web/portal/settings/cert');
      expect($node[0].querySelectorAll('.grv-settings-nav-group-menu-item').length)
        .toEqual(6, "should have correct number of menu items");

    })
  })

  describe('license feature', function () {
    const history = new createMemoryHistory();

    it('can be disabled from acl', () => {
      const acl = { licenses: { create : false } };
      setAcl(acl)
      history.push('/web/portal/settings/license')
      render(routes, history);
      expectNotActiveNavItem('Licenses');
    });

    it('can be disabled from cfg', () => {
      let acl = { license: { enabled : true } }
      setAcl(acl)
      spyOn(cfg, 'isSettingsLicenseGenEnabled').andReturn(false)
      history.push('/web/portal/settings/license')
      render(routes, history);
      expectNotActiveNavItem('Licenses');
    });
  });

});

const routes = {
 path: '/web/portal/settings',
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