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
import cfg from 'app/config';
import { APPS_INIT, APPS_ADD } from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(APPS_INIT, initApps);
    this.on(APPS_ADD, addApps);
  }
})

function addApps(state, jsonArray){
  return state.withMutations(state=>{
    jsonArray.forEach( json => {
      let itemMap = createItemMap(json);
      state.set(itemMap.get('id'), itemMap);
    });
  });
}

function initApps(state, jsonArray){
  return toImmutable({}).withMutations(state=>{
    jsonArray.forEach(item=>{
      let itemMap = createItemMap(item);
      state.set(itemMap.get('id'), itemMap);
    });
  });
}

function createItemMap(json){
  let itemMap = toImmutable(json);
  let pkgMap = itemMap.get('package');

  let name = pkgMap.get('name');
  let version = pkgMap.get('version');
  let repository = pkgMap.get('repository');
  let id = `${repository}/${name}/${version}`;
  let installUrl = cfg.getInstallNewSiteRoute(name, repository, version);
  let standaloneInstallerUrl = cfg.getStandaloneInstallerPath(name, repository, version);
  let pkgFileUrl = cfg.getAppPkgFileUrl(name, repository, version);

  itemMap = itemMap.merge({
    id,
    pkgFileUrl,
    installUrl,
    standaloneInstallerUrl
  });

  return itemMap;
}
