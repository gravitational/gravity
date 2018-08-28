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
import cfg from 'app/config';
import api from 'app/services/api';
import { fetchAppsBySite } from 'app/flux/apps/actions';
import { showSuccess, showError } from 'app/flux/notifications/actions';
import { fetchOps } from 'app/flux/operations/actions';
import * as progressActions from 'app/flux/opProgress/actions';
import restApiActions from 'app/flux/restApi/actions';
import getters from './getters';
import history from 'app/services/history';
import { RemoteAccessEnum } from 'app/services/enums';
import * as siteActions from 'app/flux/sites/actions';
import * as actionTypes from './actionTypes';
import * as featureFlags from 'app/modules/featureFlags';

import {
  TRYING_TO_START_APP_UPD_OPERATION,
  TRYING_TO_UPDATE_SITE_LICENSE,
  TRYING_TO_UPDATE_SITE_REMOTE_ACCESS } from 'app/flux/restApi/constants';

export function addNavItem(navItem) {
  reactor.dispatch(actionTypes.SITE_ADD_NAV_ITEM, navItem);
}

export function fetchSiteApps(){
  const siteId = reactor.evaluate(getters.getSiteId);
  return fetchAppsBySite(siteId);
}

export function fetchEndpoints(){
  const siteId = reactor.evaluate(getters.getSiteId);
  return api.get(cfg.getSiteEndpointsUrl(siteId))
    .done(json => {
      reactor.dispatch(actionTypes.SITE_SET_ENDPOINTS, json);
    })
}

export function fetchRemoteAccess() {
  if (!featureFlags.siteRemoteAccess()) {
    reactor.dispatch(actionTypes.SITE_REMOTE_STATUS, { status: RemoteAccessEnum.NA });
    return $.Deferred().resolve();
  }

  const siteId = reactor.evaluate(getters.getSiteId);
  return api.get(cfg.getSiteRemoteAccessUrl(siteId))
    .done(json => {
      json = json || {};
      reactor.dispatch(actionTypes.SITE_REMOTE_STATUS, json);
    })
}

export function fetchOpProgress(opId){
  const siteId = reactor.evaluate(getters.getSiteId);
  return progressActions.fetchOpProgress(siteId, opId);
}

export function openRemoteAccessDialog(){
  reactor.dispatch(actionTypes.SITE_OPEN_REMOTE_DIALOG);
}

export function closeRemoteAccessDialog(){
  reactor.dispatch(actionTypes.SITE_CLOSE_REMOTE_DIALOG);
}

export function openUpdateAppDialog(appId){
  reactor.dispatch(actionTypes.SITE_SET_APP_TO_UPDATE_TO, appId);
}

export function closeUpdateAppDialog(){
  reactor.dispatch(actionTypes.SITE_SET_APP_TO_UPDATE_TO, null);
}

export function setCurrentSiteId(siteId){
  reactor.dispatch(actionTypes.SITE_SET_ID, siteId);
}

export function changeRemoteAccess(enabled){
  const siteId = reactor.evaluate(getters.getSiteId);
  const data = {
    enabled: enabled === true
  }

  restApiActions.start(TRYING_TO_UPDATE_SITE_REMOTE_ACCESS);
  return api.put(cfg.getSiteRemoteAccessUrl(siteId), data)
    .done(json => {
      json = json || {};
      reactor.batch(() => {
        closeRemoteAccessDialog();
        restApiActions.success(TRYING_TO_UPDATE_SITE_REMOTE_ACCESS);
        reactor.dispatch(actionTypes.SITE_REMOTE_STATUS, json);

        // handle going off-line when signed-in from Ops Center
        if (!enabled) {
          redirectToClusterPage(siteId);
        }
      });
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      showError(msg, '');
      restApiActions.fail(TRYING_TO_UPDATE_SITE_REMOTE_ACCESS);
    });
}

export function updateLicense(newLicense){
  const siteId = reactor.evaluate(getters.getSiteId);
  restApiActions.start(TRYING_TO_UPDATE_SITE_LICENSE);
  siteActions.updateSiteLicense(siteId, newLicense)
    .then(siteActions.fetchSites(siteId))
    .done(() => {
      reactor.batch(()=>{
        showSuccess(`License has been updated`, '');
        restApiActions.success(TRYING_TO_UPDATE_SITE_LICENSE);
      })
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      showError(msg, '');
      restApiActions.fail(TRYING_TO_UPDATE_SITE_LICENSE);
    })
}

export function updateSiteApp(appId){
  const siteId = reactor.evaluate(getters.getSiteId);
  restApiActions.start(TRYING_TO_START_APP_UPD_OPERATION);
  siteActions.updateSite(siteId, appId)
    .then(() => {
      return $.when(fetchOps(siteId), siteActions.fetchSites(siteId));
    })
    .done(() => {
      closeUpdateAppDialog();
      restApiActions.success(TRYING_TO_START_APP_UPD_OPERATION);
      showSuccess(`Operation has been started`, '');
    })
    .fail( err => {
      let msg = api.getErrorText(err);
      showError(msg, '');
      restApiActions.fail(TRYING_TO_START_APP_UPD_OPERATION);
    })
}

export function uninstallSite(siteId, ...rest) {
  siteActions.uninstallSite(siteId, ...rest)
  .done(() => {
      redirectToUninstallPage(siteId);
  });
}

export function showTerminalNav() {
  reactor.dispatch(actionTypes.SITE_TOGGLE_TERM, true);
}

export function hideTerminalNav() {
  reactor.dispatch(actionTypes.SITE_TOGGLE_TERM, false);
}

export function redirectToClusterPage(siteId) {
  const url = cfg.getSiteRoute(siteId);
  history.push(url, true);
}

export function redirectToUninstallPage(siteId) {
  const url = cfg.getSiteUninstallRoute(siteId);
  history.push(url, true);
}