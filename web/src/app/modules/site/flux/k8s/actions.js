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
import * as actionTypes from './actionTypes';
import k8s from 'app/services/k8s';
import currentSiteGetters from './../currentSite/getters';
      
export function setSearchValue(value){
  reactor.dispatch(actionTypes.SET_SEARCH_VALUE, value);
}

export function setCurNamespace(namespace){
  reactor.dispatch(actionTypes.SET_NAMESPACE, namespace);
}

export function fetchDeployments(namespace){
  const siteId = reactor.evaluate(currentSiteGetters.getSiteId);
  return k8s.getDeployments(siteId, namespace).done(jsonArray=>{
    reactor.dispatch(actionTypes.RECEIVE_DEPLOYMENTS, jsonArray);      
  });    
}

export function fetchDaemonSets(namespace){    
  const siteId = reactor.evaluate(currentSiteGetters.getSiteId);
  return k8s.getDaemonSets(siteId, namespace).done(jsonArray=>{
    reactor.dispatch(actionTypes.RECEIVE_DAEMONSETS, jsonArray);      
  });    
}

export function fetchJobs(namespace){
  const siteId = reactor.evaluate(currentSiteGetters.getSiteId);
  return k8s.getJobs(siteId, namespace).done(jsonArray=>{
    reactor.dispatch(actionTypes.RECEIVE_JOBS, jsonArray);
  });    
}
