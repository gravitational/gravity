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

import { createMemoryHistory } from 'react-router';
import expect from 'expect';
import history from 'app/services/history';
import cfg from 'app/config';

history.init( new createMemoryHistory());

describe('services/history', function () {
  
  const fallbackRoute = cfg.routes.app;  
  const browserHistory = history.original();

  beforeEach(function () {    
    expect.spyOn(browserHistory, 'push');
    expect.spyOn(history, 'getRoutes');  
    expect.spyOn(history, '_pageRefresh');          
  });

  afterEach(function () {
    expect.restoreSpies();
  });

  describe('canPush', function () {
        
    const push = actual => ({
      andExpect(expected){
        history.push(actual)
        expect(browserHistory.push).toHaveBeenCalledWith(expected);
      }
    })
            
    it('should push if allowed else fallback to default route', function () {
      history.getRoutes.andReturn(['/valid', '/']);      
      push('invalid').andExpect(fallbackRoute);
      push('.').andExpect(fallbackRoute);
      push('/valid/test').andExpect(fallbackRoute);
      push('@#4').andExpect(fallbackRoute);
      push('/valid').andExpect('/valid');      
      push('').andExpect('');      
      push('/').andExpect('/');      
    })

    it('should refresh a page if called withRefresh=true', function () {
      let route = '/';
      history.getRoutes.andReturn([route]);            
      history.push(route, true)
      expect(history._pageRefresh).toHaveBeenCalledWith(route);
    })
  })

  describe('createRedirect()', function () {    
    it('should make valid redirect url', function () {
      let route = '/valid';
      let location = browserHistory.createLocation(route);
      history.getRoutes.andReturn([route]);      
      expect(history.createRedirect(location)).toEqual(cfg.baseUrl + route);            
    });    
  });    
})