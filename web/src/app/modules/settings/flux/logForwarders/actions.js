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
import { ResourceEnum } from 'app/services/enums';
import * as resApi from 'app/services/resources';
import api from 'app/services/api';
import * as RAT from 'app/flux/restApi/constants';
import Logger from 'app/lib/logger';
import apiActions from 'app/flux/restApi/actions';

import { getClusterName } from './../index';
import { closeDeleteDialog } from '../actions';
import { getLogForwarders } from './index';
import * as AT from './actionTypes';

const logger = Logger.create('settings/flux/forwarders/actions');

export function setCurFwrd(item) {
  reactor.batch(() => {
    apiActions.clear(RAT.TRYING_TO_SAVE_RESOURCE);
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function fetchForwarders(){
  return resApi.getForwarders(getClusterName()).done(items => {
    reactor.dispatch(AT.RECEIVE_FWRD, items);
  })
}

export function saveForwarder(logForwarder) {
  const handleError = err => {
    const msg = api.getErrorText(err);
    logger.error('saveForwarder()', err);
    apiActions.fail(RAT.TRYING_TO_SAVE_RESOURCE, msg);
  }

  const updateStore = items => reactor.batch( ()=> {
    reactor.dispatch(AT.UPDATE_FWRD, items);
    setCurFwrd(items[0].id);
    apiActions.success(RAT.TRYING_TO_SAVE_RESOURCE);
  })

  try {
    const yaml = logForwarder.getContent();
    apiActions.start(RAT.TRYING_TO_SAVE_RESOURCE);
    return resApi.upsert(getClusterName(), ResourceEnum.LOG_FWRD, yaml, logForwarder.getIsNew())
      .done(updateStore)
      .fail(handleError);
  }catch(err){
    handleError(err)
  }
}

export function deleteLogForwarder(resRec) {
  const { name, id, kind } = resRec;
  const updateStore = () => reactor.batch(()=> {
    const next = getLogForwarders().getNext(id);
    closeDeleteDialog();
    reactor.dispatch(AT.DELETE_FWRD, id);
    setCurFwrd(next);
    apiActions.success(RAT.TRYING_TO_DELETE_RESOURCE);
  });

  apiActions.start(RAT.TRYING_TO_DELETE_RESOURCE);
  resApi.remove(getClusterName(), kind, name)
    .done(updateStore)
    .fail(err => {
      const msg = api.getErrorText(err);
      logger.error('deleteLogFwrd()', err);
      apiActions.fail(RAT.TRYING_TO_DELETE_RESOURCE, msg);
    });
}
