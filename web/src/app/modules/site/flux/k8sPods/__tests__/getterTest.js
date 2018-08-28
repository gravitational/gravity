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


import reactor from 'app/reactor';
import expect from 'expect';
import { Map } from 'immutable';
import * as fakeData from 'app/__tests__/apiData'
import { receivePods } from './../actions';
import getters from './../getters';
import { setCurrentSiteId } from './../../currentSite/actions';
import './../../index';

describe('app/modules/site/flux/k8sPods/getters', () => {      
  beforeEach(() => {              
    setCurrentSiteId('any_value');
  })

  afterEach(function () {        
    reactor.reset();    
  })
            
  it('should have correct display status (waiting container)', () => {           
    let pod = new Map(fakeData.k8sPods.items[0])    
    pod = pod.merge({
      status: {
        phase: 'Test Phase',        
        containerStatuses: [{
          waiting: {
            reason: 'Test Reason',
          },
        }],
      },
    });

    receivePods([pod.toJS()]);    
    let results = reactor.evaluate(getters.podInfoList);    
    expect(results[0].statusDisplay).toBe('Waiting: Test Reason');
    expect(results[0].status).toBe('Pending');    
  });     
  

  it('should have correct display status (terminated container)', () => {       
    let pod = new Map(fakeData.k8sPods.items[0])    
    // with exist code
    pod = pod.merge({
      status: {
        phase: 'Test Phase',
        containerStatuses: [          
          {
            terminated: {              
              exitCode: '434'                          
            },
          }
        ],
      },
    });
            
    receivePods([pod.toJS()]);    
    let results = reactor.evaluate(getters.podInfoList);    
    expect(results[0].statusDisplay).toBe('Terminated: exitCode:434');
    expect(results[0].status).toBe('Terminated');    

    // with signal
    pod = pod.merge({
      status: {
        phase: 'Test Phase',
        containerStatuses: [          
          {
            terminated: {              
              signal: 'XX'                          
            },
          }
        ],
      },
    });
            
    receivePods([pod.toJS()]);    
    results = reactor.evaluate(getters.podInfoList);    
    expect(results[0].statusDisplay).toBe('Terminated: signal:XX');
    expect(results[0].status).toBe('Terminated');    
  });        

  it('should have correct display status (multi container)', () => {       
    let pod = new Map(fakeData.k8sPods.items[0])    
    pod = pod.merge({
      status: {
        phase: 'Test Phase',
        containerStatuses: [
          {
            running: { }
          },
          {
            terminated: {              
              reason: 'Test Terminated Reason'                          
            },
          },
          {
            waiting: {
              reason: 'Test Waiting Reason'
            }
          },
        ],
      },
    });
            
    receivePods([pod.toJS()]);
    
    let results = reactor.evaluate(getters.podInfoList);    
    expect(results[0].statusDisplay).toBe('Terminated: Test Terminated Reason');
    expect(results[0].status).toBe('Terminated');    
  });        

});

