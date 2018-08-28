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

import { Store, toImmutable } from 'nuclear-js';
import { RECEIVE_VALUES } from './actionTypes';
import * as SETTINGS from './../actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({
      siteId: null,
      retentionValues: {
       defVal: 0,
       medVal: 0,
       longVal: 0       
      }
    });
  },

  initialize() {
    this.on(SETTINGS.INIT, (state, { siteId } ) => state.set('siteId', siteId));
    this.on(RECEIVE_VALUES, receiveRetentionValues);    
  }
})

function receiveRetentionValues(state, values){  
  return state.mergeIn(['retentionValues'], toImmutable(values));
}
