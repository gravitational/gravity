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

import session from 'app/services/session';
import { reactor, expect, Dfd, spyOn, api, cfg } from 'app/__tests__/';
import * as actions from 'app/flux/user/actions';
import getters from 'app/flux/user/getters';
import auth from 'app/services/auth'
import history from 'app/services/history';
import { createMemoryHistory } from 'react-router';

import { AuthProviderTypeEnum } from 'app/services/enums';

describe('flux/user/actions', () => {
  
  // sample data
  const hotp = 'sample_hotpToken';
  const secretToken = 'sample_secret_token';  
  const email = 'test';
  const password = 'sample_pass';
  
  const webApiUrl = '/v1/webapi/oidc/login/web?redirect_url=:redirect&connector_id=:providerName';
  const ssoProvider = { name: 'microsoft', type: AuthProviderTypeEnum.OIDC, url: webApiUrl };

  const submitData = {    
    "password": "c2FtcGxlX3Bhc3M=",
    "second_factor_token": "sample_hotpToken",
    "token": "sample_secret_token",
    "u2f_register_response": undefined
  }

  const err = {
    responseJSON: {message: 'error'}
  }

  history.init(createMemoryHistory())

  beforeEach(function () {
    spyOn(history, 'push');       
  });

  afterEach(function () {
    reactor.reset();
    expect.restoreSpies();
  })
          
  describe('login()', function () {          
    it('should login with email', () => {                
      spyOn(auth, 'login').andReturn(Dfd().resolve());      
      actions.login(email, password);
      expect(history.push).toHaveBeenCalledWith("localhost/web", true);
    });

    it('should login with SSO', () => {                  
      const expectedUrl = `localhost/proxy/v1/webapi/oidc/login/web?redirect_url=localhost%2Fweb&connector_id=${ssoProvider.name}`;      
      actions.loginWithSso(ssoProvider.name, ssoProvider.url);                                    
      expect(history.push).toHaveBeenCalledWith(expectedUrl, true);            
    });
    
    it('should login with U2F', () => {                               
      let dummyResponse = { appId: 'xxx' }
      spyOn(api, 'post').andReturn(Dfd().resolve(dummyResponse));      
      spyOn(window.u2f, 'sign').andCall((a, b, c, d) => {        
        d(dummyResponse)
      });
        
      actions.loginWithU2f(email, password);                                    
      expect(window.u2f.sign).toHaveBeenCalled(); 
      expect(history.push).toHaveBeenCalledWith("localhost/web", true);         
    });

    it('should handle loginAttemp states', () => {
      let attemp;
      spyOn(auth, 'login').andReturn(Dfd());
      actions.login(email, password);
      
      // processing
      attemp = reactor.evaluateToJS(getters.loginAttemp);
      expect(attemp.isProcessing).toBe(true);

      // reject
      reactor.reset();
      spyOn(auth, 'login').andReturn(Dfd().reject(err));
      actions.login(email, password);
      attemp = reactor.evaluateToJS(getters.loginAttemp);
      expect(attemp.isFailed).toBe(true);
    });  
  })
      
  it('completeInviteWith2fa() should accept invite with 2FA', function () {                            
    spyOn(api, 'post').andReturn(Dfd().resolve());
    actions.completeInviteWith2fa(password, hotp, secretToken);      
    expect(api.post).toHaveBeenCalledWith(cfg.api.userTokenInviteDonePath, submitData, false);          
    expect(history.push).toHaveBeenCalledWith(cfg.routes.app, true);                                                          
  });      
  
  it('completeInviteWithU2f() should accept invite with U2F', function () {                                
    let dummyResponse = { appId: 'xxx' }
    spyOn(api, 'post').andReturn(Dfd().resolve());      
    spyOn(api, 'get').andReturn(Dfd().resolve(dummyResponse));      
    spyOn(window.u2f, 'register').andCall((a, b, c, d) => {        
      d(dummyResponse)
    });
          
    actions.completeInviteWithU2f(password, hotp, secretToken);
        
    expect(api.get).toHaveBeenCalledWith(`/proxy/v1/webapi/u2f/signuptokens/${secretToken}`);          
    expect(api.post).toHaveBeenCalledWith(cfg.api.userTokenInviteDonePath,       
      {
        "password": "c2FtcGxlX2hvdHBUb2tlbg==",
        "second_factor_token": null,
        "token": "sample_secret_token",
        "u2f_register_response": {
          "appId": "xxx"
        }
      },
      false);                    
      expect(history.push).toHaveBeenCalledWith(cfg.routes.app, true);                                                          
  });      

  it('completeResetWith2fa() should change a password with 2fa', function () {                                                           
    spyOn(api, 'post').andReturn(Dfd().resolve());
    actions.completeResetWith2fa(password, hotp, secretToken);      
    expect(api.post).toHaveBeenCalledWith(cfg.api.userTokenResetDonePath, submitData, false);                
    expect(history.push).toHaveBeenCalledWith(cfg.routes.app, true);                                           
  });

  it('should handle completeUserTokenAttempt progress indicators', () => {    
    let attemp;          
    spyOn(api, 'post').andReturn(Dfd());      
    actions.completeResetWith2fa(password, hotp, secretToken);      
    
    // processing
    attemp = reactor.evaluateToJS(getters.completeUserTokenAttemp);
    expect(attemp.isProcessing).toBe(true);

    // reject
    reactor.reset();
    spyOn(api, 'post').andReturn(Dfd().reject(err));      
    actions.completeResetWith2fa(password, hotp, secretToken);      
    attemp = reactor.evaluateToJS(getters.completeUserTokenAttemp);
    expect(attemp.isFailed).toBe(true);
  });  

  
  describe('fetchUserContext()', function () {        
    const userContext = {       
      serverVersion: {
        gitCommit : "AA",
        gitTreeState : "BB",
        version : "CC"
      },
      user: {
        email: 'test@example.com',      
        userId: 'test@example.com',
        name: 'mr Sample',
        authType: 'local',
        accountId: '111' 
      },
      userAcl: {
        authConnectors: {
          connect: false,
          list: false,
          read: false,
          edit: false,
          create: false,
          remove: false
        }
      }
    }

    it('should fetch user context', function () {      	                         
      spyOn(api, 'get').andReturn(Dfd().resolve(userContext));                  
      actions.fetchUserContext();
      
      const actualUser = reactor.evaluateToJS(['user']);
      const actualUserAcl = reactor.evaluateToJS(['useracl']);
      
      actions.fetchUserContext();
      expect(api.get).toHaveBeenCalledWith(cfg.api.userContextPath);
      expect(actualUser).toEqual(userContext.user);
      expect(actualUserAcl.authConnectors).toEqual(userContext.userAcl.authConnectors);
      expect(cfg.getServerVersion()).toEqual(userContext.serverVersion);
    })

    it('should redirect to woopsy if failed', function () {      	                         
      const errorText = 'error!'      
      spyOn(api, 'get').andReturn(Dfd().reject( new Error(errorText)));
      spyOn(history, 'push');

      actions.fetchUserContext();
      expect(history.push).toHaveBeenCalledWith(cfg.getWoopsyPageRoute(errorText));
    })

  })

  describe('ensureUser()', function () {        
    const userContext = {       
      serverVersion: {},
      user: {},
      userAcl: {}
    }

    it('should ensure the session and retrive user context', function () {      
      const cb = expect.createSpy(()=>{});
      spyOn(api, 'get').andReturn(Dfd().resolve(userContext));
      spyOn(session, 'ensureSession').andReturn( Dfd().resolve());

      actions.ensureUser(null, null, cb);
      expect(cb).toHaveBeenCalled();
      expect(session.ensureSession).toHaveBeenCalled();
    });

    it('should navigate to login page and set a redirect URL if session is not valid', function () {      
      spyOn(session, 'ensureSession').andReturn(Dfd().reject(err));
      const cb = expect.createSpy(()=>{});
      const replace = expect.createSpy(()=>{});
      const location = {
        pathname: 'xxx'
      };
            
      actions.ensureUser({location}, replace, cb);
      expect(cb).toHaveBeenCalled();
      expect(replace).toHaveBeenCalledWith({
        "pathname": "/web/login",
        "search": "?redirect_uri=localhost/web"
      });
    });
  });
})
