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
import api from 'app/services/api';
import cfg from 'app/config';
import restApiActions from 'app/flux/restApi/actions';
import Logger from 'app/lib/logger';
import { getClusterName } from './../index';
import { checkIfNotNil } from 'app/lib/paramUtils';
import { UserStatusEnum } from 'app/services/enums'

import accountGetters from './getters';
import { TRYING_TO_RESET_USER, TRYING_TO_INVITE, TRYING_TO_UPDATE_USER, TRYING_TO_DELETE_USER } from 'app/flux/restApi/constants';

import {
  SETTINGS_USRS_SET_NEW,
  SETTINGS_USRS_SET_CURRENT,
  SETTINGS_USRS_SET_USER_TO_DELETE,
  SETTINGS_USRS_SET_USER_TO_REINVITE,
  SETTINGS_USRS_SET_USER_TO_RESET,
  SETTINGS_USRS_RECEIVE_USERS,
  SETTINGS_USRS_CLEAR
 }  from './actionTypes';

const logger = Logger.create('modules/account/actions');

export function clear() {
  reactor.dispatch(SETTINGS_USRS_CLEAR);    
  restApiActions.clear(TRYING_TO_INVITE)
}

export function cancelAddEditUser() {
  reactor.dispatch(SETTINGS_USRS_SET_CURRENT, null)        
  clearAttempts();
}

export function addUser() {
  reactor.dispatch(SETTINGS_USRS_SET_NEW)
}

export function openEditUserDialog(id) {
  reactor.dispatch(SETTINGS_USRS_SET_CURRENT, id)
}

export function clearAttempts(){
  reactor.batch( () => {
    restApiActions.clear(TRYING_TO_INVITE);
    restApiActions.clear(TRYING_TO_UPDATE_USER);    
    restApiActions.clear(TRYING_TO_RESET_USER);
  });
}

export function createInvite(user){
  const data = {  
    name: user.userId,
    roles: user.roles
  };
  
  restApiActions.start(TRYING_TO_INVITE);            
  return api.post(cfg.getSiteUserInvitePath(getClusterName()), data)
    .done(userToken => {    
      refresh();           
      restApiActions.success(TRYING_TO_INVITE, userToken);              
    })
    .fail(err => {
      let msg = api.getErrorText(err);    
      logger.error('saveUser()', err);        
      restApiActions.fail(TRYING_TO_INVITE, msg);        
  })    
}

export function resetUser(userId) {            
  restApiActions.start(TRYING_TO_RESET_USER);            
  return api.post(cfg.getSiteUserResetPath(getClusterName(), userId))
    .done(userToken => {    
      refresh();
      restApiActions.success(TRYING_TO_RESET_USER, userToken);              
    })
    .fail(err => {
      let msg = api.getErrorText(err);    
      logger.error('saveUser()', err);        
      restApiActions.fail(TRYING_TO_RESET_USER, msg);        
  })
}

export function saveUser(user) {
  restApiActions.start(TRYING_TO_UPDATE_USER);            
  return api.put(cfg.api.accountUsersPath, user)
    .done(userToken => {
      refresh();     
      cancelAddEditUser();
      restApiActions.success(TRYING_TO_UPDATE_USER, userToken);              
    })
    .fail(err => {
      let msg = api.getErrorText(err);    
      logger.error('saveUser()', err);        
      restApiActions.fail(TRYING_TO_UPDATE_USER, msg);        
    });    
}

export function openResetUserDialog(userId){
  reactor.dispatch(SETTINGS_USRS_SET_USER_TO_RESET, userId);
}

export function closeResetUserDialog(){
  reactor.dispatch(SETTINGS_USRS_SET_USER_TO_RESET, null);    
  clearAttempts();
}

export function openDeleteUserDialog(userId) {
  reactor.dispatch(SETTINGS_USRS_SET_USER_TO_DELETE, userId);
}

export function closeDeleteUserDialog(){
  reactor.dispatch(SETTINGS_USRS_SET_USER_TO_DELETE, null);
  clearAttempts();
}

export function openResendInviteDialog(user) {
  reactor.dispatch(SETTINGS_USRS_SET_USER_TO_REINVITE, user);
}

export function closeResendInviteDialog(){
  reactor.dispatch(SETTINGS_USRS_SET_USER_TO_REINVITE, null);
  clearAttempts();
}
  
export function reInviteUserUser(userId) {
  const store = reactor.evaluate(accountGetters.userStore);
  const user = store.users.find(u => u.userId === userId)    
  createInvite(user);      
}

export function deleteUser(userId) {
  checkIfNotNil(userId, 'userId');
  let usersStore = reactor.evaluate(accountGetters.userStore);

  let isInvite = usersStore.users.some(
    item => item.userId === userId && item.status === UserStatusEnum.INVITED);

  let url = isInvite ?
    cfg.getAccountDeleteInviteUrl(userId) : cfg.getAccountDeleteUserUrl(userId);

  restApiActions.start(TRYING_TO_DELETE_USER);
  api.delete(url)
    .done(()=> {
      refresh();
      restApiActions.success(TRYING_TO_DELETE_USER);
      closeDeleteUserDialog();
    })
    .fail( err=> {
      let msg = api.getErrorText(err);
      restApiActions.fail(TRYING_TO_DELETE_USER, msg);
      logger.error('deleteUser()', err);        
    });
}

export function fetchUsers() {
  return api.get(cfg.api.accountUsersPath).done(users => {
    reactor.dispatch(SETTINGS_USRS_RECEIVE_USERS, users);  
  })
}

function refresh() {
  fetchUsers().fail(err => {      
    logger.error('fetchUsers()', err);      
  });
}