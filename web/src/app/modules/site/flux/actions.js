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
import currentSiteGetters from './currentSite/getters';
import { SiteStateEnum } from 'app/services/enums';
import history from 'app/services/history';
import cfg from 'app/config';
import $ from 'jQuery';
import Logger from 'app/lib/logger';
import restApiActions from 'app/flux/restApi/actions';
import * as featureFlags from './../../featureFlags';

// actions
import { createTerminal } from './masterConsole/actions';
import { fetchOps } from 'app/flux/operations/actions';
import { fetchSites, applyConfig } from 'app/flux/sites/actions';
import {
  redirectToUninstallPage,
  setCurrentSiteId,
  fetchSiteApps,
  fetchRemoteAccess,
  fetchEndpoints
} from './currentSite/actions';

import { TRYING_TO_INIT_APP } from 'app/flux/restApi/constants';
import { SITE_INIT } from './actionTypes';
import siteGetters from 'app/flux/sites/getters';

const logger = Logger.create('site/flux/actions');

export function initCluster(siteId, featureActivator) {
  setCurrentSiteId(siteId);
  restApiActions.start(TRYING_TO_INIT_APP);
  return $.when(
    fetchRemoteAccess(),
    fetchSiteApps(),
    fetchOps(siteId),
    fetchSites(siteId),
    fetchEndpoints()
  )
  .done(() => {
    try {
      let state = reactor.evaluate(siteGetters.siteStateById(siteId));
      if (state === SiteStateEnum.UNINSTALLING) {
        redirectToUninstallPage(siteId);
        return;
      }

      applyConfig(siteId);
      reactor.dispatch(SITE_INIT, { siteId });

      featureActivator.onload({
        siteId,
        featureFlags
      });

      restApiActions.success(TRYING_TO_INIT_APP);
    } catch (err) {
      handleInitError(err);
    }
  })
  .fail(handleInitError);
}

export function fetchSite(){
  let siteId = getSiteId();
  return fetchSites(siteId);
}

export function fetchSiteOps(){
  let siteId = getSiteId();
  return fetchOps(siteId);
}

export function openServerTerminal(params){
  let siteId = getSiteId();
  let newRoute = cfg.getSiteConsoleRoute(siteId);
  reactor.batch(() => {
    createTerminal(params);
    history.push(newRoute);
  });
}

function getSiteId(){
  return reactor.evaluate(currentSiteGetters.getSiteId);
}

function handleInitError(err) {
  logger.error('site init', err);
  let msg = api.getErrorText(err);
  restApiActions.fail(TRYING_TO_INIT_APP, msg);
}