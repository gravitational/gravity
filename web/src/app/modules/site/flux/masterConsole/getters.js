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
import { getNodes }  from 'app/flux/nodes';
import currentSiteGetters from './../currentSite/getters';

const lastTermKey = [['site_master_console'], masterConsoleMap => {
  return masterConsoleMap.get('terminals').keySeq().last();
}]

const masterConsole = [ ['site_master_console'], ['nodes'], (masterConsoleMap) => {
  let terminals = masterConsoleMap
    .get('terminals')
    .valueSeq()
    .map(getTerminalData)
    .toJS();

  return {
    terminals,
    activeTerminal: masterConsoleMap.get('activeTerminal'),
    isVisible: masterConsoleMap.get('isVisible'),
    isInitialized: masterConsoleMap.get('isInitialized')
  };
}];

function getTerminalMapByServerId(key){
  return reactor.evaluate(['site_master_console', 'terminals', key]);
}

function getTerminalData(termMap) {
  let key = termMap.get('key');
  let login = termMap.get('login');
  let serverId = termMap.get('serverId');
  let pod = termMap.get('pod');
  let currentSite = reactor.evaluate(currentSiteGetters.currentSite());
  let { role, ip } = getNodes().findServer(serverId) || {};
  let hostname = `${serverId}`;

  if (pod && pod.name) {
    hostname = pod.name;
  }else if(role || ip){
    hostname = `${role} ${ip}`
  }

  let title = `${login}@${hostname}`;

  return {
    key,
    clusterName: currentSite.domainName,
    login,
    title,
    pod,
    serverId
  }
}

export default {
  lastTermKey,
  masterConsole,
  getTerminalMapByServerId
}
