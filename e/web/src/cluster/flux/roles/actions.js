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
import Logger from 'oss-app/lib/logger';
import * as AT from './actionTypes';

const logger = Logger.create('flux/roles/actions');

export function saveRole(yaml, isNew) {
  const handleError = err => {
    logger.error('saveRole()', err);
  }

  const updateStore = items => {
    reactor.dispatch(AT.UPSERT_ROLES, items);
  }

  return resApi.upsert(ResourceEnum.ROLE, yaml, isNew)
    .done(updateStore)
    .fail(handleError)
}

export function deleteRole(roleRec) {
  const { name, id } = roleRec;
  const updateStore = () => {
    reactor.dispatch(AT.DELETE_ROLE, id);
  }

  return resApi.remove(ResourceEnum.ROLE, name)
    .then(updateStore)
    .fail(err => {
      logger.error('deleteRole()', err);
    });
}

export function fetchRoles() {
  return resApi.getRoles().done(items => {
    reactor.dispatch(AT.RECEIVE_ROLES, items);
  })
}