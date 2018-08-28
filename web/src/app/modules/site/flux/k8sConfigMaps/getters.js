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

import { toImmutable } from 'nuclear-js';
import {requestStatus} from 'app/flux/restApi/getters';
import { TRYING_TO_UPDATE_SITE_CONFIG_MAP } from 'app/flux/restApi/constants';

const configMaps = [ ['site_config_maps'], (configMap) =>{
  let selectedNamespaceName = configMap.get('selected_namespace_name');
  let selectedConfigName = configMap.get('selected_config_name');
  let configs = configMap.get('configs')
    .filter(itemMap => itemMap.getIn(['metadata', 'namespace']) === selectedNamespaceName)
    .map(itemMap => {
      let metadata =  itemMap.get('metadata') || toImmutable({});
      let {name, uid, namespace} = metadata.toJS();
      let data = getDataItems(itemMap.get('data'));

      return {
        name,
        id: uid,
        namespace,
        data
      }
    }).toJS();

  return {
    selectedNamespaceName,
    selectedConfigName,
    configs,
    namespaceNames: configMap.get('available_namespaces'),
    isDirty: configMap.get('is_dirty')    
  }
 }
];

function getDataItems(dataMap){
  let data = []

  if(dataMap){
    dataMap.toKeyedSeq().forEach((item, key)=>{
      data.push({
        name: key,
        content: item
      })
    });
  }

  return data;
}

export default {
  configMaps,
  updateCfgMapAttemp: requestStatus(TRYING_TO_UPDATE_SITE_CONFIG_MAP)
}
