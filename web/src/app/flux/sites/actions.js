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

import $ from 'jQuery';
import reactor from 'app/reactor';
import api from 'app/services/api';
import cfg from 'app/config';
import restApiActions from 'app/flux/restApi/actions';
import { showError } from 'app/flux/notifications/actions';

import { SITES_RECEIVE_DATA, SITES_APPLY_CONFIG, SITES_OPEN_CONFIRM_DELETE, SITES_CLOSE_CONFIRM_DELETE } from './actionTypes';
import { TRYING_TO_DELETE_SITE, FETCHING_SITES } from 'app/flux/restApi/constants';
import { ClusterNameEnum } from 'app/services/enums';

import devClusterJson from './devCluster.json';

export function openSiteConfirmDelete(siteId){
  reactor.dispatch(SITES_OPEN_CONFIRM_DELETE, siteId)
}

export function closeSiteConfirmDelete(){
    reactor.dispatch(SITES_CLOSE_CONFIRM_DELETE);
    restApiActions.clear(TRYING_TO_DELETE_SITE);
  }

export function applyConfig(siteId){
  reactor.dispatch(SITES_APPLY_CONFIG, siteId);
}

export function fetchFlavors(siteId){
  return api.get(cfg.getSiteFlavorsUrl(siteId));
}

export function updateSiteLicense(siteId, newLicense){
  let data = {
    license: newLicense
  }

  return api.put(cfg.getSiteLicenseUrl(siteId), data);
}

export function updateSite(siteId, appId){
  let [repo, name, version] = appId.split('/');
  let data = {
    package: `${repo}/${name}:${version}`
  }

  return api.put(cfg.getSiteUrl(siteId), data);
}

export function uninstallAndDeleteSite(siteId, data){
  return api.delete(cfg.getSiteUrl(siteId), data);
}

export function uninstallSite(siteId, secretKey, accessKey, sessionToken){
  var variables = { 'secret_key': secretKey, 'access_key': accessKey, 'session_token': sessionToken };
  var data = {
    force: true,
    variables
  }

  restApiActions.start(TRYING_TO_DELETE_SITE);
  return uninstallAndDeleteSite(siteId, data)
    .done(()=>{
      closeSiteConfirmDelete();
      restApiActions.success(TRYING_TO_DELETE_SITE);
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      showError(msg, '');
      restApiActions.fail(TRYING_TO_DELETE_SITE, msg);
    });
}

export function fetchSites(siteId){
  // when run locally, provide a dummy Ops Center Cluster manifest
  if(siteId === ClusterNameEnum.DevCluster){
    reactor.dispatch(SITES_RECEIVE_DATA, [devClusterJson]);
    return $.Deferred().resolve([devClusterJson]);
  }

  return api.get(cfg.getSiteUrl(siteId))
    .then( json =>{
      json = Array.isArray(json) ? json : [json];

      // fixme: as we clear all site data on SITES_RECEIVE_DATA, we need to ensure
      // devCluster id stays in the store thus lets insert it in each response for now.
      if (cfg.isDevCluster()) {
        json.push(devClusterJson);
      }

      return json;
    })
    .done((jsonArray)=>{
      reactor.dispatch(SITES_RECEIVE_DATA, jsonArray);
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      restApiActions.fail(FETCHING_SITES, msg);
    });
}

