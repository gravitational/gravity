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

import React from 'react';
import * as ReactDOM from 'react-dom';
import { Login } from 'app/components/user/login';
import { Provider as ReactorProvider } from 'nuclear-js-react-addons';
import * as actions from 'app/flux/user/actions';
import { Auth2faTypeEnum, AuthProviderTypeEnum } from 'app/services/enums';
import { $, expect, spyOn, cfg, reactor } from 'app/__tests__/';
import { makeHelper } from 'app/__tests__/domUtils'
import 'assets/js/jquery-validate';

const $node = $('<div>');
const helper = makeHelper($node);

describe('components/components/user/login', function () {

  beforeEach(() => {
    helper.setup();
    spyOn(actions, 'login');
    spyOn(actions, 'loginWithU2f');
    spyOn(actions, 'loginWithSso');
    spyOn(cfg, 'getAuthProviders').andReturn([]);
    spyOn(cfg, 'getAuth2faType').andReturn(Auth2faTypeEnum.DISABLED)
  })

  afterEach(function () {
    helper.clean();
    expect.restoreSpies();
    reactor.reset();
  })

  const webApiUrl = '/v1/webapi/oidc/login/web?redirect_url=:redirect&connector_id=:providerName';
  const ssoProvider = { name: 'microsoft', type: AuthProviderTypeEnum.OIDC, url: webApiUrl };
  const email = 'email@email';
  const password = 'samplePassword';
  const token = 'token'

  it('should login using basic auth', () => {
    render();
    helper.setText($node.find('input[name="userId"]')[0], email);
    helper.setText($node.find('input[name="password"]')[0], password);
    expectNInputs(3);

    clickLogin()
    expect(actions.login).toHaveBeenCalledWith(email, password, '');
  });

  it('should login with Auth2faTypeEnum.UTF', () => {
    cfg.getAuth2faType.andReturn(Auth2faTypeEnum.UTF)

    render();
    helper.setText($node.find('input[name="userId"]')[0], email);
    helper.setText($node.find('input[name="password"]')[0], password);
    expectNInputs(3);

    clickLogin()
    expect(actions.loginWithU2f).toHaveBeenCalledWith(email, password);
  })

  it('should login with Auth2faTypeEnum.OTP', () => {
    cfg.getAuth2faType.andReturn(Auth2faTypeEnum.OTP)

    render();
    helper.setText($node.find('input[name="userId"]')[0], email);
    helper.setText($node.find('input[name="password"]')[0], password);
    helper.setText($node.find('input[name="token"]')[0], token);
    expectNInputs(4);

    clickLogin()
    expect(actions.login).toHaveBeenCalledWith(email, password, token);
  })

  it('should login with SSO', () => {
    cfg.getAuth2faType.andReturn(Auth2faTypeEnum.DISABLED)
    cfg.getAuthProviders.andReturn([ssoProvider])

    render();
    expectNInputs(4);
    $node.find('.btn-microsoft').click();
    expect(actions.loginWithSso).toHaveBeenCalledWith(ssoProvider.name, ssoProvider.url);
  })

  it('should properly render UI elements', () => {
    cfg.getAuth2faType.andReturn(Auth2faTypeEnum.OTP)
    render();
    expect($node.find('h2').text()).toBe('Gravity');
    helper.shouldExist('.grv-icon-gravitational-logo');
    helper.shouldExist('.grv-google-auth');
  })

  it('should validate the input (email, psw, and a token) and warn about errors', () => {
    cfg.getAuth2faType.andReturn(Auth2faTypeEnum.OTP)
    render();
    $node.find(".btn-primary").click();
    expect($node.find('#userId-error, #password-error, #token-error').length).toBe(3);
  });
});

const render = props => {
  let attemp = {}
  props = props || { attemp }
  ReactDOM.render((
    <ReactorProvider reactor={reactor}>
      <Login {...props}  />
    </ReactorProvider>)
    , $node[0]);

}

const clickLogin = () => {
  $node.find(".btn-primary").click();
}

const expectNInputs = n => {
  expect($node.find('input, button').length).toBe(n);
}