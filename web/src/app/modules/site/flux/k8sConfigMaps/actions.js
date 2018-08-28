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
import k8s from 'app/services/k8s';
import {showSuccess, showError} from 'app/flux/notifications/actions';
import currentSiteGetters from './../currentSite/getters';
import restApiActions from 'app/flux/restApi/actions';
import { TRYING_TO_UPDATE_SITE_CONFIG_MAP } from 'app/flux/restApi/constants';
import {
  SITE_RECEIVE_CONFIG_MAPS,
  SITE_CONFIG_MAPS_INIT,
  SITE_CONFIG_MAPS_SET_SELECTED_NAMESPACE,
  SITE_CONFIG_MAPS_SET_SELECTED_CFG_NAME,
  SITE_CONFIG_MAPS_SET_DIRTY
 } from './actionTypes';

export function initCfgMaps(){    
  reactor.dispatch(SITE_CONFIG_MAPS_INIT);    
}

export function saveConfigMap(namespace, configName, data){
  var siteId = reactor.evaluate(currentSiteGetters.getSiteId);
  restApiActions.start(TRYING_TO_UPDATE_SITE_CONFIG_MAP);
  k8s.saveConfigMap(siteId, namespace, configName, {data})
    .done(()=>{
      reactor.batch(()=>{
        fetchCfgMaps();
        makeDirty(false);
        restApiActions.success(TRYING_TO_UPDATE_SITE_CONFIG_MAP);
        showSuccess(`${namespace}/${configName} has been updated`, '');
      })
    })
    .fail(()=>{
      showError(`Failed to update ${namespace}/${configName}`, '');
      restApiActions.fail(TRYING_TO_UPDATE_SITE_CONFIG_MAP);
    })
}

export function makeDirty(value){
  reactor.dispatch(SITE_CONFIG_MAPS_SET_DIRTY, value);
}

export function setSelectedNamespace(value){
  reactor.dispatch(SITE_CONFIG_MAPS_SET_SELECTED_NAMESPACE, value);
}

export function setSelectedCfgName(value){
  reactor.dispatch(SITE_CONFIG_MAPS_SET_SELECTED_CFG_NAME, value);
}

export function fetchCfgMaps(){
  var siteId = reactor.evaluate(currentSiteGetters.getSiteId);
  return k8s.getConfigMaps(siteId).done((cfgMapsItemArray)=>{
    reactor.dispatch(SITE_RECEIVE_CONFIG_MAPS, cfgMapsItemArray);
  })
}
