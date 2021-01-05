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

import reactor from 'oss-app/reactor';
import { ResourceEnum } from 'oss-app/services/enums';
import * as resApi from 'oss-app/services/resources';
import Logger from 'shared/libs/logger';
import * as AT from './actionTypes';

const logger = Logger.create('flux/settingsAuth/actions');

export function fetchAuthProviders(){
  return resApi.getAuthProviders().done(json => {
    reactor.dispatch(AT.RECEIVE_CONNECTORS, json);
  })
}

export function saveAuthProvider(yaml, isNew) {
  return resApi.upsert(ResourceEnum.AUTH_CONNECTORS, yaml, isNew)
    .done(items => {
      reactor.dispatch(AT.UPDATE_CONNECTORS, items);
    })
    .fail(err => {
      logger.error('saveAuthProvider()', err);
    });
}

export function deleteAuthProvider(authRec) {
  const { name, id, kind } = authRec;
  return resApi.remove(kind, name)
    .done(()=>{
      reactor.dispatch(AT.DELETE_CONN, id);
    })
    .fail(err => {
      logger.error('deleteAuthProvider()', err);
    });
}
