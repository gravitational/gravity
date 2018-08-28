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
import api from 'app/services/api';
import cfg from 'app/config';
import {fetchNodes} from 'app/flux/nodes/actions';
import currentSiteGetters from './../currentSite/getters';
import termGetters from './getters';
import * as currentSiteActions from '../currentSite/actions';
import history from 'app/services/history';

import {
  SITE_MASTER_CONSOLE_INIT,
  SITE_MASTER_CONSOLE_INIT_TERMINAL,
  SITE_MASTER_CONSOLE_REMOVE_TERMINAL,
  SITE_MASTER_CONSOLE_HIDE,
  SITE_MASTER_CONSOLE_SET_CURRENT_TERMINAL,
  SITE_MASTER_CONSOLE_SHOW } from './actionTypes';

export function createNewSession(login, domainName){
  let data = { 'session': {'terminal_params': {'w': 45, 'h': 5}, login}}
  return api.post(cfg.getSiteSessionUrl(domainName), data);
}

export function removeTerminal(key){
  reactor.dispatch(SITE_MASTER_CONSOLE_REMOVE_TERMINAL, key);
  let termKey = reactor.evaluate(termGetters.lastTermKey);
  if (!termKey) {
    history.goBack(-1);      
  }
}

export function setActiveTerminal(key){
  reactor.dispatch(SITE_MASTER_CONSOLE_SET_CURRENT_TERMINAL, key);    
}

export function createTerminal(params){
  reactor.dispatch(SITE_MASTER_CONSOLE_INIT_TERMINAL, params);
  let termKey = reactor.evaluate(termGetters.lastTermKey);
  setActiveTerminal(termKey);    
}

export function onEnterTerminalPage(nextState, replace) {
  let termKey = reactor.evaluate(termGetters.lastTermKey);
  if (termKey) {
    currentSiteActions.showTerminalNav(true); 
  } else {
    let { siteId } = nextState.params;             
    replace(cfg.getSiteServersRoute(siteId));
  } 
}

export function onLeaveTerminalPage() {
  let termKey = reactor.evaluate(termGetters.lastTermKey);
  if (!termKey) {
    currentSiteActions.hideTerminalNav(false);           
  }    
}

export function initMasterConsole(){
  let currentSite = reactor.evaluate(currentSiteGetters.currentSite());
  fetchNodes(currentSite.domainName).done(()=>{
    reactor.dispatch(SITE_MASTER_CONSOLE_INIT);
  });
}

// terminal host (div) action
export function hideTerminal(){
  reactor.dispatch(SITE_MASTER_CONSOLE_HIDE);
}

// terminal host (div) action  
export function showTerminal(){
  reactor.batch(()=>{
    reactor.dispatch(SITE_MASTER_CONSOLE_SHOW);
  });
}
