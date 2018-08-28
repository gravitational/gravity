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
import opGetters from 'app/flux/operations/getters';
import currentSiteGetters from './../currentSite/getters';
import {
  SITE_HISTORY_SET_CURRENT,
  SITE_HISTORY_INIT } from './actionTypes';

export function init(){
  let siteId = reactor.evaluate(currentSiteGetters.getSiteId);
  let ops = reactor.evaluate(opGetters.opsBySiteId(siteId));
  let selectedOpId = null;

  if(ops.length > 0){
    selectedOpId = ops[0].get('id');
  }

  reactor.batch(()=>{
    reactor.dispatch(SITE_HISTORY_SET_CURRENT, selectedOpId);
    reactor.dispatch(SITE_HISTORY_INIT);
  })
}

export function setSelectedOpId(opId){
  reactor.dispatch(SITE_HISTORY_SET_CURRENT, opId);
}