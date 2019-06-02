/*
Copyright 2019 Gravitational, Inc.

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
import cfg from 'app/config';
import api from 'app/services/api';
import { SITE_SET_REMOTE_STATUS, SITE_RECEIVE_INFO } from './actionTypes';

export function fetchSiteInfo(){
  return api.get(cfg.getSiteInfoUrl()).then(json => {
    reactor.dispatch(SITE_RECEIVE_INFO, json)
  })
}

export function fetchRemoteAccess() {
  return api.get(cfg.getSiteRemoteAccessUrl())
    .done(json => {
      json = json || {};
      reactor.dispatch(SITE_SET_REMOTE_STATUS, json);
    })
}

export function changeRemoteAccess(enabled){
  const data = {
    enabled: enabled === true
  }

  return api.put(cfg.getSiteRemoteAccessUrl(), data)
    .done(json => {
      json = json || {};
      reactor.dispatch(SITE_SET_REMOTE_STATUS, json);
    })
}