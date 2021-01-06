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
import reactor from 'oss-app/reactor';
import { StoreRec } from 'oss-app/modules/settings/flux/records';
import * as AT from './actionTypes';

export function getAuthStore() {
  return reactor.evaluate(['tlp_settings_auth'])
}

export default Store({

  getInitialState() {
    return new StoreRec()
  },

  initialize() {
    this.on(AT.UPDATE_CONNECTORS, (state, items) => state.upsertItems(items) );
    this.on(AT.RECEIVE_CONNECTORS, (state, items) => state.setItems(items) );
    this.on(AT.SET_CURRENT, (state, item) => state.setCurItem(item))
    this.on(AT.DELETE_CONN, (state, id) => state.remove(id));
  }
})

