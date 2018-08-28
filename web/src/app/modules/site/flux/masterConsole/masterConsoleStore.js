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

import { Store, toImmutable, Immutable } from 'nuclear-js';
import { Record } from 'immutable';
import {
  SITE_MASTER_CONSOLE_HIDE,
  SITE_MASTER_CONSOLE_INIT,
  SITE_MASTER_CONSOLE_INIT_TERMINAL,
  SITE_MASTER_CONSOLE_SET_CURRENT_TERMINAL,
  SITE_MASTER_CONSOLE_REMOVE_TERMINAL,
  SITE_MASTER_CONSOLE_SHOW }  from './actionTypes';

const DEFAULT_LOGIN = 'root';

class TerminalRequestRec extends Record({
  key: false,
  serverId: '',
  login: '',
  pod: null,  
}){
  constructor(props){
    const login = props.login || DEFAULT_LOGIN;
    const key = Math.random().toString();    
    super({
      ...props,
      key,
      login      
    })
  }
}

export default Store({
  getInitialState() {
    return toImmutable({
      isInitialized: false,
      isVisible: false,
      activeTerminal: 0,
      terminals: new Immutable.OrderedMap()
    });
  },

  initialize() {
    this.on(SITE_MASTER_CONSOLE_SHOW, state => state.set('isVisible', true));
    this.on(SITE_MASTER_CONSOLE_HIDE, state => state.set('isVisible', false));
    this.on(SITE_MASTER_CONSOLE_INIT, initMasterConsole);
    this.on(SITE_MASTER_CONSOLE_INIT_TERMINAL, initTerminal);
    this.on(SITE_MASTER_CONSOLE_SET_CURRENT_TERMINAL, setActiveTerminal);
    this.on(SITE_MASTER_CONSOLE_REMOVE_TERMINAL, removeTerminal);
  }
})

function initMasterConsole(state){  
  return state.set('isInitialized', true);
}

function removeTerminal(state, key) {
  return  state.deleteIn(['terminals', key]);  
}

function setActiveTerminal(state, key){
  return state.set('activeTerminal', key);
}

function initTerminal(state, json){      
  const termReq = new TerminalRequestRec(json);    
  return state.setIn(['terminals', termReq.key], termReq);  
}
