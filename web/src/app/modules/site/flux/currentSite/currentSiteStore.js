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

import cfg from 'app/config';
import {RemoteAccessEnum} from 'app/services/enums';
import {Store, toImmutable} from 'nuclear-js';
import * as actionTypes from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({
      id: null,
      nav: [],
      endpoints: null,
      appToUpdateTo: null,
      remoteAccess: RemoteAccessEnum.OFF,
      remoteAccessDialogOpen: false
    });
  },

  initialize() {
    this.on(actionTypes.SITE_SET_ID, setCurrentSiteId);
    this.on(actionTypes.SITE_REMOTE_STATUS, setRemoteStatus);
    this.on(actionTypes.SITE_TOGGLE_TERM, showHideTerminalNavItem);
    this.on(actionTypes.SITE_ADD_NAV_ITEM, addNavItem);
    this.on(actionTypes.SITE_SET_APP_TO_UPDATE_TO, (state, appId) =>
      state.set('appToUpdateTo', appId));
    this.on(actionTypes.SITE_SET_ENDPOINTS, (state, endpoints) =>
      state.set('endpoints', toImmutable(endpoints)));
    this.on(actionTypes.SITE_OPEN_REMOTE_DIALOG, state =>
      state.set('remoteAccessDialogOpen', true));
    this.on(actionTypes.SITE_CLOSE_REMOTE_DIALOG, state =>
      state.set('remoteAccessDialogOpen', false));
  }
})

function showHideTerminalNavItem(state, isVisible) {
  let siteId = state.get('id');
  return state.updateIn(['nav'], navList => {
    return navList.map(itemMap => {
      if (itemMap.get('key') === 'servers') {
        if (isVisible) {
          return itemMap.set('children', toImmutable([createConsoleNavItem(siteId)]))
        } else {
          return itemMap.delete('children');
        }

      }
      return itemMap;
    })
  })
}

const createConsoleNavItem = siteId => ({
  icon: 'fa fa-terminal',
  to: cfg.getSiteConsoleRoute(siteId),
  title: 'Console'
})

function addNavItem(state, navItem) {
  const nav = state
    .get('nav')
    .push(toImmutable(navItem));
  return state.set('nav', nav);
}

function setRemoteStatus(state, {status}) {
  return state.set('remoteAccess', status);
}

function setCurrentSiteId(state, siteId) {
  return state.set('id', siteId);
}
