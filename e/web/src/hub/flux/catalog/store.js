/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import { keyBy } from 'lodash';
import * as actionTypes from './actionTypes';

const CatalogStoreRec = Record({
  apps: {},
});

export default Store({
  getInitialState() {
    return new CatalogStoreRec();
  },

  initialize() {
    this.on(actionTypes.CATALOG_RECEIVE_APPS, receiveApps);
  }
})

function receiveApps(state, json) {
  const apps = keyBy(json, 'id');
  return state.set('apps', apps);
}