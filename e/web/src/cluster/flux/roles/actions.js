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