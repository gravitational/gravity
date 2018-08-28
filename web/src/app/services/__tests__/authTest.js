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

import { expect, $, api, cfg, spyOn } from 'app/__tests__/';
import auth from 'app/services/auth';
import history from 'app/services/history';

describe('services/auth', () => {
  
  const sample = { token: 'token' };

  beforeEach(() => {
    spyOn(api, 'post');
    spyOn(history, 'push');
  });

  afterEach( () => {
    expect.restoreSpies();
  })

  describe('login({user, password, token, provider, redirect})', () => {        
    const email = 'alex';
    const password = 'password';
    const token = 'xxx';
    
    it('should login with email, password, and token', () => {
      const postParams = {
        pass: password,
        user: email,
        second_factor_token: token
      }
            
      api.post.andReturn($.Deferred().resolve(sample));
      auth.login(email, password, token);                  
      expect(api.post).toHaveBeenCalledWith(cfg.api.sessionPath, postParams, false);              
    });
    
    it('should return rejected promise if failed to log in', () => {
      var wasCalled = false;
      api.post.andReturn($.Deferred().reject());
      auth.login(email, password).fail(()=> { wasCalled = true });
      expect(wasCalled).toEqual(true);
    });

  });  
})
