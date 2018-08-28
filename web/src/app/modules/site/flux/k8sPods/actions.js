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
import currentSiteGetters from './../currentSite/getters';
import { SITE_RECEIVE_PODS }  from './actionTypes';

export function receivePods(json){
  reactor.dispatch(SITE_RECEIVE_PODS, json);
}

export function fetchPods(namespace /*optional*/){
  const siteId = reactor.evaluate(currentSiteGetters.getSiteId);
  return k8s.getPods(siteId, namespace).done(receivePods);
}

