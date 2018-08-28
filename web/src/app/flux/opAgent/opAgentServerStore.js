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
  OP_AGENT_SERVERS_INIT,
  OP_AGENT_SERVERS_SET_VARS,
  OP_AGENT_SERVERS_CLEAR } from './actionTypes';

const defaultState = () =>
  toImmutable({    
    servers: {
      /*
        serverRole: [ server_vars, server_vars  ]
      */
    }
  });

export default Store({
  getInitialState() {
    return defaultState();
  },

  initialize() {
    this.on(OP_AGENT_SERVERS_CLEAR, removeServerConfigs)
    this.on(OP_AGENT_SERVERS_SET_VARS, setServerConfigs)
    this.on(OP_AGENT_SERVERS_INIT, init)
  }
})

function removeServerConfigs(state, serverRole){
  return state.deleteIn(['servers', serverRole])
}

function setServerConfigs(state, { serverRole, serverConfigs } ){
  return state.setIn(['servers', serverRole], toImmutable(serverConfigs) );
}

function init(){  
  return defaultState();
}
