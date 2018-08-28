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

import $ from 'jQuery';
import reactor from 'app/reactor';
import auth from 'app/services/auth';
import session from 'app/services/session';
import restApiActions from 'app/flux/restApi/actions';
import {showSuccess} from 'app/flux/notifications/actions';
import cfg from 'app/config';
import api from 'app/services/api';
import {checkIfNotNil} from 'app/lib/paramUtils';
import Logger from 'app/lib/logger';
import history from 'app/services/history';
import {
  FETCHING_USER_TOKEN,
  TRYING_TO_LOGIN,  
  TRYING_TO_COMPLETE_USER_TOKEN,
  TRYING_TO_CHANGE_PSW
   } from 'app/flux/restApi/constants';

import { USER_CONFIRM_RECEIVE_DATA, USER_RECEIVE_DATA } from './actionTypes';

import { USERACL_RECEIVE } from './../userAcl/actionTypes';

const logger = Logger.create('modules/user/actions');

export function logout(){
  session.logout();
}

export function login(userId, password, token) {            
  checkIfNotNil(userId, 'userName');
  checkIfNotNil(password, 'password');
  let promise = auth.login(userId, password, token);      
  _handleLoginPromise(promise);             
}

export function loginWithU2f(userId, password) {
  let promise = auth.loginWithU2f(userId, password);
  _handleLoginPromise(promise);
}

export function loginWithSso(providerName, redirectUrl) {
  let appStartRoute = getEntryRoute();    
  let ssoUri = cfg.getSsoUrl(redirectUrl, providerName, appStartRoute);
  history.push(ssoUri, true);                  
}

export function fetchUserContext(){
  return api.get(cfg.api.userContextPath).done(json=>{      
    cfg.setServerVersion(json.serverVersion);
    reactor.dispatch(USER_RECEIVE_DATA, json.user);
    reactor.dispatch(USERACL_RECEIVE, json.userAcl);
  })
  .fail(err => {
    let text = api.getErrorText(err);
    history.push(cfg.getWoopsyPageRoute(text));
  })
}

export function ensureUser(nextState, replace, cb){        
  $.when(session.ensureSession())
    .done(()=> {
      fetchUserContext().always(()=>{
        cb();
      });
    })
    .fail(()=>{                                
      // store original URL for redirect
      let uri = history.createRedirect(nextState.location);        
      let search = `?redirect_uri=${uri}`;        
      // navigate to login
      replace({
        pathname: cfg.routes.login,
        search
      });
      
      cb();
    });
}

export function changePassword(oldPsw, newPsw){
  checkIfNotNil(oldPsw, 'old password');
  checkIfNotNil(newPsw, 'new password');

  let data = {
    'old_password': window.btoa(oldPsw),
    'new_password': window.btoa(newPsw)
  }

  restApiActions.start(TRYING_TO_CHANGE_PSW);
  api.post(cfg.api.changeUserPswPath, data)
    .done(()=>{
      restApiActions.success(TRYING_TO_CHANGE_PSW);
      showSuccess(`Your password has been changed`);
    })
    .fail(err => {
      logger.error('Failed to change password', err);
      let msg = api.getErrorText(err)
      restApiActions.fail(TRYING_TO_CHANGE_PSW, msg);
    });
}

export function completeInviteWithU2f(userId, password, tokenId) {
  const promise = auth.getU2FRegisterRes(tokenId).then(u2fRes => {
    return completeUserToken(cfg.api.userTokenInviteDonePath, tokenId, password, null, u2fRes)
  })    

  return _handleCompleteTokenPromise(promise);
}
  
export function completeInviteWith2fa(psw, hotpToken, tokenId) {                
  const promise = completeUserToken(
    cfg.api.userTokenInviteDonePath,
    tokenId,
    psw,
    hotpToken      
  );        

  _handleCompleteTokenPromise(promise);
}

export function completeResetWith2fa(psw, hotpToken, tokenId) {        
  const promise = completeUserToken(
    cfg.api.userTokenResetDonePath,
    tokenId,
    psw,
    hotpToken,      
  );        

  _handleCompleteTokenPromise(promise);
}
  
export function completeResetWithU2f(userId, password, tokenId) {
  const promise = auth.getU2FRegisterRes(tokenId).then(u2fRes => {
    return completeUserToken(
      cfg.api.userTokenResetDonePath, 
      tokenId, password, 
      null, 
      u2fRes
    );
  })    

  _handleCompleteTokenPromise(promise);
}

export function fetchUserToken(tokenId){
  var path = cfg.getUserRequestInfo(tokenId);
  restApiActions.start(FETCHING_USER_TOKEN);
  api.get(path)
    .done(confirmInfo=>{
      reactor.dispatch(USER_CONFIRM_RECEIVE_DATA, confirmInfo);
      restApiActions.success(FETCHING_USER_TOKEN);
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      restApiActions.fail(FETCHING_USER_TOKEN, msg);        
    });
}


const getEntryRoute = () => {    
  let entryUrl = history.getRedirectParam();
  if (entryUrl) {
    entryUrl = history.ensureKnownRoute(entryUrl);
  } else {
    entryUrl = cfg.routes.app;
  }
  
  return history.ensureBaseUrl(entryUrl);
}

const _handleLoginPromise = promise => {
  restApiActions.start(TRYING_TO_LOGIN);
  return promise.done(() => {        
      let redirect = getEntryRoute();
      history.push(redirect, true);        
    })
    .fail(err => {
      logger.error('login', err);
      let msg = api.getErrorText(err)
      restApiActions.fail(TRYING_TO_LOGIN, msg);
    });    
}

const _handleCompleteTokenPromise = promise => {
  restApiActions.start(TRYING_TO_COMPLETE_USER_TOKEN);
  return promise
    .done(()=>{
      restApiActions.success(TRYING_TO_COMPLETE_USER_TOKEN);
      history.push(cfg.routes.app, true);    
    })
    .fail(err=>{
      let msg = api.getErrorText(err);
      restApiActions.fail(TRYING_TO_COMPLETE_USER_TOKEN, msg);
    })
}

const completeUserToken = (url, tokenId, psw, hotpToken, u2fResponse) => {  
  checkIfNotNil(url, 'url');
  checkIfNotNil(tokenId, 'tokenId');
  checkIfNotNil(psw, 'psw');

  const data = {
    'password': window.btoa(psw),
    'second_factor_token': hotpToken,
    'token': tokenId,
    'u2f_register_response': u2fResponse
  }
    
  return api.post(url, data, false);  
}
