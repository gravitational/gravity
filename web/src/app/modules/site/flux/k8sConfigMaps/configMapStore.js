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

import namespaceGetters from './../k8sNamespaces/getters';
import reactor from 'app/reactor';
import { Store, toImmutable } from 'nuclear-js';
import {
  SITE_CONFIG_MAPS_INIT,
  SITE_RECEIVE_CONFIG_MAPS,
  SITE_CONFIG_MAPS_SET_SELECTED_NAMESPACE,
  SITE_CONFIG_MAPS_SET_SELECTED_CFG_NAME,
  SITE_CONFIG_MAPS_SET_DIRTY
} from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({
      selected_namespace_name: null,
      selected_config_name: null,      
      is_dirty: false,
      opened_config: null,
      available_namespaces: [],
      configs: []
    });
  },

  initialize() {
    this.on(SITE_RECEIVE_CONFIG_MAPS, receiveCfgMaps);
    this.on(SITE_CONFIG_MAPS_INIT, init);
    this.on(SITE_CONFIG_MAPS_SET_SELECTED_NAMESPACE, setSelectedNamespace);
    this.on(SITE_CONFIG_MAPS_SET_SELECTED_CFG_NAME, setSelectedCfgName);
    this.on(SITE_CONFIG_MAPS_SET_DIRTY, makeDirty);
  }
})

function makeDirty(state, value){
  return state.set('is_dirty', value);
}

function setSelectedCfgName(state, selectedConfigName){
  return state.set('selected_config_name', selectedConfigName);
}

function setSelectedNamespace(state, selectedNamespaceName){
  let selectedConfigName = getFirstNamespaceConfig(state, selectedNamespaceName);
  return state.set('selected_namespace_name', selectedNamespaceName)
              .set('selected_config_name', selectedConfigName)
}

function init(state){
  let selectedNamespaceName;
  let namespaceNames = reactor.evaluate(namespaceGetters.namespaceNames);

  if(namespaceNames.length > 0){
    selectedNamespaceName = namespaceNames[0];
  }

  state = setSelectedNamespace(state, selectedNamespaceName);

  return state.set('is_dirty', false)
              .set('available_namespaces', namespaceNames)
}

function getFirstNamespaceConfig(state, namespaceName){
  let cfgMap = state.get('configs').find(
    itemMap => itemMap.getIn(['metadata', 'namespace']) === namespaceName
  );

  if(cfgMap){
    return cfgMap.getIn(['metadata', 'name']);
  }else{
    return null;
  }
}

function receiveCfgMaps(state, jsonArray){
  return state.set('configs', toImmutable(jsonArray));
}
