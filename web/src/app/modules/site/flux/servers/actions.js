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
import {showSuccess, showError} from 'app/flux/notifications/actions';
import currentSiteGetters from './../currentSite/getters';
import restApiActions from 'app/flux/restApi/actions';
import getters from './getters';
import opAgentActions from 'app/flux/opAgent/actions';
import { createExpand, createShrink, fetchOps, deleteOp, startOp } from 'app/flux/operations/actions';
import api from 'app/services/api';
import cfg from 'app/config';
import { ProviderEnum, ProvisionerEnum } from 'app/services/enums';
import opAgentGetters from 'app/flux/opAgent/getters';

import {
  SITE_SERVERS_DLG_SET_SRV_TO_DELETE,
  SITE_SERVERS_PROV_INIT,
  SITE_SERVERS_PROV_SET_INSTANCE_TYPE,
  SITE_SERVERS_PROV_SET_PROFILE,
  SITE_SERVERS_PROV_RESET,
  SITE_SERVERS_PROV_SET_NODE_COUNT,
  SITE_SERVERS_RECEIVE } from './actionTypes';

import {
  TRYING_TO_DELETE_EXPAND_OPERATION,
  TRYING_TO_CREATE_EXPAND_OPERATION,
  TRYING_TO_START_EXPAND_OPERATION,
  TRYING_TO_START_SHRINK_OPERATION } from 'app/flux/restApi/constants';

export function initWithNewServer(){
  let {provider} = reactor.evaluate(currentSiteGetters.currentSite());
  let needKeys = provider === ProviderEnum.AWS;
  reactor.dispatch(SITE_SERVERS_PROV_INIT, {
    needKeys,
    isNewServer: true,
    isExistingServer: false
  });
}

export function initWithExistingServer(){
  reactor.dispatch(SITE_SERVERS_PROV_INIT, {
    isExistingServer: true,
    isNewServer: false
  });
}

export function createExpandOperation(accessKey, secretKey, sessionToken){
  let {isExistingServer, selectedProfileKey} = reactor.evaluate(getters.serverProvision);
  let {provider, id} = reactor.evaluate(currentSiteGetters.currentSite());
  let provisioner = isExistingServer ? ProvisionerEnum.ONPREM : '';
  let data = {
    profile: selectedProfileKey,
    provider: {
      provisioner,
      [provider]: {
        access_key: accessKey,
        secret_key: secretKey,
        session_token: sessionToken
      }
    }
  }

  restApiActions.start(TRYING_TO_CREATE_EXPAND_OPERATION);

  createExpand(id, data)
    .done(()=>{
      restApiActions.success(TRYING_TO_CREATE_EXPAND_OPERATION);
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      restApiActions.fail(TRYING_TO_CREATE_EXPAND_OPERATION, msg);
      showError(msg, '');
    })
}

export function cancelExpandOperation(){
  let { initiatedOpId, siteId } = reactor.evaluate(getters.serverProvision);

  if(!initiatedOpId){
    reactor.dispatch(SITE_SERVERS_PROV_RESET);
    return;
  }

  restApiActions.start(TRYING_TO_DELETE_EXPAND_OPERATION);
  deleteOp(siteId, initiatedOpId)
    .done(()=> {
      restApiActions.success(TRYING_TO_DELETE_EXPAND_OPERATION);
      reactor.dispatch(SITE_SERVERS_PROV_RESET);
    })
    .fail( err => {
      let msg = api.getErrorText(err);
      showError(msg, '');
      restApiActions.fail(TRYING_TO_DELETE_EXPAND_OPERATION);
    })
}

export function startOperation(){
  let {initiatedOpId, isNewServer, siteId} = reactor.evaluate(getters.serverProvision);
  let data = {};

  if(isNewServer){
    let {instanceType, nodeCount, selectedProfileKey} = reactor.evaluate(getters.newServer);
    data.profiles = {};
    data.profiles[selectedProfileKey] = {
      instance_type: instanceType,
      count: nodeCount
    }
  }else{
    data.servers = reactor.evaluate(opAgentGetters.serverConfigs);
  }

  restApiActions.start(TRYING_TO_START_EXPAND_OPERATION);
  return startOp(siteId, initiatedOpId, data)
    .done(() => {
      fetchOps(siteId);
      reactor.dispatch(SITE_SERVERS_PROV_RESET);
      restApiActions.success(TRYING_TO_START_EXPAND_OPERATION);
      showMessage()
    })
    .fail(err => {
      let msg = api.getErrorText(err);
      showError(msg, '');
      restApiActions.fail(TRYING_TO_START_EXPAND_OPERATION, msg);
    })
}

export function showRemoveServerConfirmation(hostname){
  reactor.dispatch(SITE_SERVERS_DLG_SET_SRV_TO_DELETE, hostname);
}

export function hideRemoveServerConfirmation(){
  reactor.dispatch(SITE_SERVERS_DLG_SET_SRV_TO_DELETE, null);
}

export function startShrinkOperation({ hostname, secretKey, accessKey, sessionToken }){
  let { provider, id } = reactor.evaluate(currentSiteGetters.currentSite());
  let data = {
    servers: [hostname],
    provider: {
      [provider]: {
        secret_key: secretKey,
        access_key: accessKey,
        session_token: sessionToken
      }
    }
  };

  restApiActions.start(TRYING_TO_START_SHRINK_OPERATION);
  createShrink(id, data)
    .done(()=>{
      restApiActions.success(TRYING_TO_START_SHRINK_OPERATION);
      hideRemoveServerConfirmation();
      showMessage();
    })
    .fail(err => {
      restApiActions.fail(TRYING_TO_START_SHRINK_OPERATION);
      let msg = api.getErrorText(err);
      showError(msg, ``);
    });
}

export function setNodeCount(value){
  reactor.dispatch(SITE_SERVERS_PROV_SET_NODE_COUNT, value);
}

export function setProfile(value){
  reactor.dispatch(SITE_SERVERS_PROV_SET_PROFILE, value);
}

export function setInstanceType(instanceType){
  reactor.dispatch(SITE_SERVERS_PROV_SET_INSTANCE_TYPE, instanceType);
}

export function fetchAgentReport(){
  let {initiatedOpId, siteId} = reactor.evaluate(getters.serverProvision);
  return opAgentActions.fetchReport(siteId, initiatedOpId);
}

export function fetchServers(){
  var siteId = reactor.evaluate(currentSiteGetters.getSiteId);
  return api.get(cfg.getSiteServersUrl(siteId))
    .done(serverDataArray=>{
      reactor.dispatch(SITE_SERVERS_RECEIVE, serverDataArray);
    });
}

function showMessage() {
  showSuccess(`Operation has been started`, '');
}