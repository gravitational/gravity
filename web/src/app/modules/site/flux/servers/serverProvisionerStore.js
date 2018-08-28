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
import {
  SITE_SERVERS_PROV_INIT,
  SITE_SERVERS_PROV_RESET,
  SITE_SERVERS_PROV_SET_INSTANCE_TYPE,
  SITE_SERVERS_PROV_SET_PROFILE,
  SITE_SERVERS_PROV_SET_NODE_COUNT
 } from './actionTypes';

const defaultState = toImmutable({
    isNewServer: false,
    isExistingServer: false,
    needKeys: false,
    instanceType: null,
    selectedProfileKey: null,
    nodeCount: 1,
    opId: null
  });

export default Store({
  getInitialState() {
    return defaultState;
  },

  initialize() {
    this.on(SITE_SERVERS_PROV_INIT, initAddNewServerOperation);
    this.on(SITE_SERVERS_PROV_RESET, () => defaultState);
    this.on(SITE_SERVERS_PROV_SET_INSTANCE_TYPE, setInstanceType);
    this.on(SITE_SERVERS_PROV_SET_PROFILE, setProfile);
    this.on(SITE_SERVERS_PROV_SET_NODE_COUNT, setNodeCount);
  }
})

function setInstanceType(state, type){
  return state.set('instanceType', type);
}

function setProfile(state, value){
  return state.set('selectedProfileKey', value)
              .set('nodeCount', 1)
              .set('instanceType', null);
}

function setNodeCount(state, value){
  return state.set('nodeCount', value);
}

function initAddNewServerOperation(state, {isNewServer, isExistingServer, needKeys}){
  return state.set('isNewServer', isNewServer)
              .set('needKeys', needKeys)
              .set('isExistingServer', isExistingServer)
}
