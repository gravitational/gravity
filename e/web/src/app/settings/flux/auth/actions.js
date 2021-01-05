import reactor from 'oss-app/reactor';
import { ResourceEnum } from 'oss-app/services/enums';
import * as resApi from 'oss-app/services/resources';
import api from 'oss-app/services/api';
import * as RAT from 'oss-app/flux/restApi/constants';
import Logger from 'oss-app/lib/logger';
import apiActions from 'oss-app/flux/restApi/actions';
import { getClusterName } from 'oss-app/modules/settings/flux/index';
import { closeDeleteDialog } from 'oss-app/modules/settings/flux/actions';

// local
import { getAuthSettings } from './index';
import * as AT from './actionTypes';

const logger = Logger.create('flux/settingsAuth/actions');

export function setCurProvider(item) {
  reactor.batch(() => {
    apiActions.clear(RAT.TRYING_TO_SAVE_RESOURCE);
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function fetchAuthProviders(){
  return resApi.getAuthProviders(getClusterName()).done(items => {
    reactor.dispatch(AT.RECEIVE_CONNECTORS, items);
  })
}

export function saveAuthProvider(authProvider) {
  const handleError = err => {
    const msg = api.getErrorText(err);
    logger.error('saveAuthProvider()', err);
    apiActions.fail(RAT.TRYING_TO_SAVE_RESOURCE, msg);
  }

  const updateStore = items => reactor.batch( ()=> {
    reactor.dispatch(AT.UPDATE_CONNECTORS, items);
    setCurProvider(items[0].id);
    apiActions.success(RAT.TRYING_TO_SAVE_RESOURCE);
  })

  try {
    const yaml = authProvider.getContent();
    apiActions.start(RAT.TRYING_TO_SAVE_RESOURCE);
    return resApi.upsert(getClusterName(), ResourceEnum.AUTH_CONNECTORS, yaml, authProvider.getIsNew())
      .done(updateStore)
      .fail(handleError);
  }catch(err){
    handleError(err)
  }
}

export function deleteAuthProvider(authRec) {
  const { name, id, kind } = authRec;

  const updateStore = () => reactor.batch(()=> {
    const next = getAuthSettings().getNext(id);
    closeDeleteDialog();
    reactor.dispatch(AT.DELETE_CONN, id);
    setCurProvider(next);
    apiActions.success(RAT.TRYING_TO_DELETE_RESOURCE);
  });

  apiActions.start(RAT.TRYING_TO_DELETE_RESOURCE);
  resApi.remove(getClusterName(), kind, name)
    .done(updateStore)
    .fail(err => {
      const msg = api.getErrorText(err);
      logger.error('deleteAuthProvider()', err);
      apiActions.fail(RAT.TRYING_TO_DELETE_RESOURCE, msg);
    });
}
