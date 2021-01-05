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
import api from 'oss-app/services/api';
import { TRYING_TO_SAVE_RESOURCE, TRYING_TO_DELETE_RESOURCE } from 'oss-app/flux/restApi/constants';
import { ResourceEnum } from 'oss-app/services/enums';
import * as resApi from 'oss-app/services/resources';
import Logger from 'oss-app/lib/logger';
import apiActions from 'oss-app/flux/restApi/actions';
import { getClusterName } from 'oss-app/modules/settings/flux/index';
import { closeDeleteDialog } from 'oss-app/modules/settings/flux/actions';

import { getRoles } from './index';
import * as AT from './actionTypes';

const logger = Logger.create('flux/settingsCluster/actions');

export function setCurRole(item) {
  reactor.batch(() => {
    apiActions.clear(TRYING_TO_SAVE_RESOURCE);
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function saveRole(rolRec) {
  const handleError = err => {
    const msg = api.getErrorText(err);
    logger.error('saveRole()', err);
    apiActions.fail(TRYING_TO_SAVE_RESOURCE, msg);
  }

  const updateStore = items => reactor.batch(()=> {
    reactor.dispatch(AT.UPSERT_ROLES, items);
    setCurRole(items[0].id);
    apiActions.success(TRYING_TO_SAVE_RESOURCE);
  })

  try {
    const yaml = rolRec.getContent();
    apiActions.start(TRYING_TO_SAVE_RESOURCE);
    return resApi.upsert(getClusterName(), ResourceEnum.ROLE, yaml, rolRec.getIsNew())
      .done(updateStore)
      .fail(handleError)
  }
  catch(err){
    handleError(err);
  }
}

export function deleteRole(rolRec) {
  const { name, id } = rolRec;
  const updateStore = () => {
    reactor.batch(()=>{
      const next = getRoles().getNext(id);
      closeDeleteDialog();
      reactor.dispatch(AT.DELETE_ROLE, id);
      apiActions.success(TRYING_TO_DELETE_RESOURCE);
      setCurRole(next);
    })
  }

  apiActions.start(TRYING_TO_DELETE_RESOURCE);
  resApi.remove(getClusterName(), ResourceEnum.ROLE, name)
    .done(updateStore)
    .fail(err => {
      const msg = api.getErrorText(err);
      logger.error('deleteRole()', err);
      apiActions.fail(TRYING_TO_DELETE_RESOURCE, msg);
    });
}

export function fetchRoles() {
  return resApi.getRoles(getClusterName()).done(items => {
    reactor.dispatch(AT.RECEIVE_ROLES, items);
  })
}
