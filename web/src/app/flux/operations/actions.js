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
import $ from 'jQuery';
import { OPS_RECEIVE, OPS_ADD, OPS_DELETE } from './actionTypes';

export function createShrink(siteId, data) {    
  return api.post(cfg.getShrinkSiteUrl(siteId), data)
    .then( json => {
      reactor.dispatch(OPS_ADD, json.operation);
    })  
}

export function createExpand(siteId, data){
  return api.post(cfg.getExpandSiteUrl(siteId), data)
    .then( json => {
      reactor.dispatch(OPS_ADD, json.operation);
    })
}

export function startOp(siteId, opId, data){
  return api.post(cfg.getOperationStartUrl(siteId, opId), data)
}

export function deleteOp(siteId, opId) {
  return api.delete(cfg.getOperationUrl(siteId, opId))
    .then(()=>{
      reactor.dispatch(OPS_DELETE, opId);
  })
}

export function fetchOps(siteId, opId) {
  let url = cfg.getOperationUrl(siteId, opId);
  return api.get(url).then( data => {
    try{
      let opDataArray = Array.isArray(data) ? data : [data];
      reactor.dispatch(OPS_RECEIVE, opDataArray);
    }catch(err){
      return $.Deferred().reject(err);
    }
  });
}