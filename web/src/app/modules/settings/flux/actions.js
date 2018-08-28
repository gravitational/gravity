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
import apiActions from 'app/flux/restApi/actions';
import api from 'app/services/api';
import Logger from 'app/lib/logger';
import * as RAT from 'app/flux/restApi/constants';
import * as AT from './actionTypes';
import { fetchSites, applyConfig } from 'app/flux/sites/actions';
const logger = Logger.create('settings/flux/actions');

export function addNavItem(groupName, navItem){
  reactor.dispatch(AT.ADD_NAV_ITEM, {
    groupName,
    navItem
  })
}

export function initSettings(activationContext, featureActivator) {
  apiActions.start(RAT.TRYING_TO_INIT_APP);
  const { siteId } = activationContext;
  return $.when(fetchSites(activationContext.siteId))
    .then(() => {
      applyConfig(siteId);
      reactor.dispatch(AT.INIT, activationContext);
      featureActivator.onload(activationContext)
      apiActions.success(RAT.TRYING_TO_INIT_APP);
    })
    .fail(err => {
      logger.error('initSettings', err);
      let msg = api.getErrorText(err);
      apiActions.fail(RAT.TRYING_TO_INIT_APP, msg);
    })
}

export function openDeleteDialog(item){
  reactor.dispatch(AT.SET_RES_TO_DELETE, item);
}

export function closeDeleteDialog(){
  apiActions.clear(RAT.TRYING_TO_DELETE_RESOURCE);
  reactor.dispatch(AT.SET_RES_TO_DELETE, null);
}