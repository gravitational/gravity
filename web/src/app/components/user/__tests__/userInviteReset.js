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
import  { Dfd, $, expect, reactor, api, spyOn, cfg } from 'app/__tests__/';
import { Provider as ReactorProvider } from 'nuclear-js-react-addons';
import { Auth2faTypeEnum, UserTokenTypeEnum } from 'app/services/enums';
import * as actions from 'app/flux/user/actions';
import UserInviteReset from 'app/components/user/userInviteReset';
import { makeHelper } from 'app/__tests__/domUtils'
import 'assets/js/jquery-validate';

const tokenSecret = require('app/__tests__/apiData/tokenSecret.json')
const $node = $('<div>');
const helper = makeHelper($node);

describe('components/components/user/userInviteReset (invite)', () => {

  const inviteJson = {
    ...tokenSecret,
    type: UserTokenTypeEnum.INVITE
  }

  const password = 'samplePassword';
  const token = 'token'
  const userId = tokenSecret.user;

  beforeEach(() => {
    helper.setup();
    spyOn(api, 'get').andReturn(Dfd().resolve(inviteJson));
    spyOn(actions, 'completeInviteWith2fa');
    spyOn(actions, 'completeInviteWithU2f');
    spyOn(cfg, 'getAuth2faType').andReturn(Auth2faTypeEnum.DISABLED)
  })

  afterEach(() => {
    helper.clean()
    expect.restoreSpies();
    reactor.reset();
  })

  it('should validate input fields', () => {
    spyOn(cfg, 'getAuth2faType').andReturn(Auth2faTypeEnum.OTP)
    render();
    $node.find(".btn-primary").click();
    expect($node.find('#password-error, #passwordConfirmed-error, #token-error').length).toBe(3);
  });

  it('should handle submit without 2FA', () => {
    spyOn(cfg, 'getAuth2faType').andReturn(Auth2faTypeEnum.DISABLED)
    render();
    expectRenderWithout2FA()

    setPasswordFields(password)
    clickSubmit()
    expectNInputs(4)
    expect(actions.completeInviteWith2fa).toHaveBeenCalledWith(password, '', tokenSecret.token);
  })

  it('should handle submit with 2FA (OTP)', () => {
    spyOn(cfg, 'getAuth2faType').andReturn(Auth2faTypeEnum.OTP)
    render();
    expectRenderOTP(tokenSecret)

    setPasswordFields(password)
    setTokenField(token)
    clickSubmit()

    expectNInputs(5)
    expect(actions.completeInviteWith2fa).toHaveBeenCalledWith(password, token, tokenSecret.token);
  });

  it('should handle submit with 2FA (U2F)', () => {
    spyOn(cfg, 'getAuth2faType').andReturn(Auth2faTypeEnum.UTF)
    render();
    expectRenderUTF()

    setPasswordFields(password)
    clickSubmit()

    expectNInputs(4)
    expect(actions.completeInviteWithU2f).toHaveBeenCalledWith(userId, password, tokenSecret.token);
  });

})

describe('components/components/user/newUserRequest (change password)', () => {
  const passwordResetJson = {
    ...tokenSecret,
    type: UserTokenTypeEnum.RESET
  }

  const password = 'samplePassword';
  const token = 'token'

  beforeEach(() => {
    helper.setup();
    spyOn(api, 'get').andReturn(Dfd().resolve(passwordResetJson));
    spyOn(actions, 'completeResetWith2fa');
    spyOn(actions, 'completeInviteWithU2f');
    spyOn(cfg, 'getAuth2faType').andReturn(Auth2faTypeEnum.DISABLED)
  })

  afterEach(() => {
    helper.clean()
    expect.restoreSpies();
    reactor.reset();
  })

  it('should display correct title', () => {
    render();
    expect($node.find('h3:first').text().trim()).toBe('User Reset');
  });

  it('should handle submit without 2FA', () => {
    spyOn(cfg, 'getAuth2faType').andReturn(Auth2faTypeEnum.DISABLED)
    render();
    expectRenderWithout2FA()

    setPasswordFields(password)
    clickSubmit()
    expectNInputs(4)
    expect(actions.completeResetWith2fa).toHaveBeenCalledWith(password, '', tokenSecret.token);
  })

  it('should handle submit with 2FA (OTP)', () => {
    spyOn(cfg, 'getAuth2faType').andReturn(Auth2faTypeEnum.OTP)
    render();
    expectRenderOTP(tokenSecret)

    setPasswordFields(password)
    setTokenField(token)
    clickSubmit()

    expectNInputs(5)
    expect(actions.completeResetWith2fa).toHaveBeenCalledWith(password, token, tokenSecret.token);
  });
})

const setPasswordFields = pass => {
  helper.setText($node.find('input[name="password"]')[0], pass);
  helper.setText($node.find('input[name="passwordConfirmed"]')[0], pass);
}

const setTokenField = token => {
  helper.setText($node.find('input[name="token"]')[0], token);
}

const expectRenderWithout2FA = () => {
  helper.shouldNotExist('.grv-invite-utf-info')
  helper.shouldNotExist('.grv-google-auth');
  helper.shouldNotExist('.grv-invite-barcode')
}

const expectRenderOTP = (requestInfo) => {
  var src = $node.find('.grv-invite-barcode img').attr('src');
  expect(src).toContain(requestInfo.qr_code);
  helper.shouldExist('.grv-google-auth');
  helper.shouldNotExist($node,'.grv-invite-utf-info')
}

const expectRenderUTF = () => {
  helper.shouldNotExist('.grv-google-auth');
  helper.shouldNotExist('.grv-invite-barcode');
  expect($node.find('.grv-invite-utf-info').length).toBe(2);
}

function render(){
  ReactDOM.render((
    <ReactorProvider reactor={reactor}>
      <UserInviteReset params={{"token": "xxxx"}} />
    </ReactorProvider>)
    , $node[0]);

}

const clickSubmit = () => {
  $node.find(".btn-primary").click();
}

const expectNInputs = n => {
  expect($node.find('input, button').length).toBe(n);
}