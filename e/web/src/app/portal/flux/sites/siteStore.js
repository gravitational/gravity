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

import { Store, toImmutable } from 'nuclear-js';
import { PORTAL_SITE_OPEN_CONFIRM_UNLINK, PORTAL_SITE_CLOSE_CONFIRM_UNLINK   } from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({
      siteToDelete: null,
      siteToUnlink: null
    });
  },

  initialize() {    
    this.on(PORTAL_SITE_OPEN_CONFIRM_UNLINK, (state, siteId) => state.set('siteToUnlink', siteId) );
    this.on(PORTAL_SITE_CLOSE_CONFIRM_UNLINK, state => state.set('siteToUnlink', null) );
  }
})
