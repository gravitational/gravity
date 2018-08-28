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
import { APPS_INIT, APPS_ADD } from './actionTypes';
import { RepositoryEnum } from 'app/services/enums';

function handleServerResponse(json){
  json = json || [];
  json = Array.isArray(json) ? json : [json];
  reactor.dispatch(APPS_INIT, json);
}

export function addApps(json){
  json = Array.isArray(json) ? json : [json];
  reactor.dispatch(APPS_ADD, json);
}

export function deleteApp(appId){
  let [repository, name, version] = appId.split('/');
  return api.delete(cfg.getAppsUrl(name, repository, version));
}

export function fetchAppsBySite(siteId){
  return api.get(cfg.getSiteAppsUrl(siteId)).done(handleServerResponse);
}

export function fetchApps(name, repository=RepositoryEnum.SYSTEM, version){
  return api.get(cfg.getAppsUrl(name, repository, version))
    .done(handleServerResponse);
}