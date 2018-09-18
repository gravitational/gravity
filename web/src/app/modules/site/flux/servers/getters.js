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
import cfg from 'app/config';
import opGetters from 'app/flux/operations/getters';
import helpers from 'app/flux/operations/helpers';
import siteGetters from 'app/flux/sites/getters';
import { ProvisionerEnum, ProviderEnum, ExpandPolicyEnum } from 'app/services/enums';
import { requestStatus } from 'app/flux/restApi/getters';
import * as userAclFlux from 'app/flux/userAcl';

import {
  TRYING_TO_START_SHRINK_OPERATION,
  TRYING_TO_DELETE_EXPAND_OPERATION,
  TRYING_TO_START_EXPAND_OPERATION,
  TRYING_TO_CREATE_EXPAND_OPERATION } from 'app/flux/restApi/constants';

// cloud
const newServer = [
  ['site_servers_provisioner'],
  ['site_current', 'id'], (provMap, siteId) => {

  let profiles = createProfiles(siteId);
  let selectedProfileKey = provMap.get('selectedProfileKey');
  let numberOfNodesOptions = [1, 2, 3, 4, 5];
  let instanceTypeOptions = [];

  if(profiles[selectedProfileKey]){
    instanceTypeOptions = ensureAllowedInstanceTypes(
      selectedProfileKey,
      profiles[selectedProfileKey]);
  }

  return {
    profiles,
    numberOfNodesOptions,
    instanceTypeOptions,
    instanceType: provMap.get('instanceType'),
    selectedProfileKey: provMap.get('selectedProfileKey'),
    nodeCount: provMap.get('nodeCount'),
    needKeys: provMap.get('needKeys')
  }
}]

// onprem
const existingServer =  [
  ['site_servers_provisioner'],
  ['site_current', 'id'], (provMap, siteId) => {

  let selectedProfileKey = provMap.get('selectedProfileKey');
  let profiles = createProfiles(siteId);

  return {
    selectedProfileKey,
    profiles
  }
}]

const existingServerPendingOperation = opId => [
  ['site_current', 'id'], (siteId) => {

  let instructionsMap = reactor.evaluate(opGetters.getOnPremInstructions(opId));

  if(!instructionsMap){
    return null;
  }

  let profileKey = instructionsMap.keySeq().first()
  let instructions = instructionsMap.getIn([profileKey, 'instructions']);
  let profiles = createProfiles(siteId);
  let profile = profiles[profileKey];

  return {
    profile,
    instructions
  }
}]

const serverToRemove = ['site_servers_dialogs', 'srvToDelete'] ;

const serverProvision = [
  ['site_servers_provisioner'],
  ['site_current', 'id'],
  ['op'], (provMap, siteId) => {

  let {provider} = reactor.evaluate(siteGetters.siteById(siteId));
  let serverOps = reactor.evaluate(opGetters.serverOps(siteId));
  let isNewServer = provMap.get('isNewServer');
  let isExistingServer = provMap.get('isExistingServer');
  let inProgressOpId = null;
  let initiatedOpId = null;
  let inProgressOpType = null;
  let selectedProfileKey = provMap.get('selectedProfileKey');
  let isOpInProgress = false;
  let historyUrl = cfg.getSiteHistoryRoute(siteId);

  if(serverOps.length > 0){
    let opMap = serverOps[0];
    let state = opMap.get('state');
    let opId = opMap.get('id');

    if (helpers.isInProgress(state)) {
      inProgressOpId = opId;
      inProgressOpType = opMap.get('type')
    }else if(helpers.isInitiated(state)){
      isExistingServer = opMap.get('provisioner') === ProvisionerEnum.ONPREM;
      isNewServer = !isExistingServer;
      initiatedOpId = opId;
    }
  }

  return {
    siteId,
    provider,
    inProgressOpId,
    initiatedOpId,
    inProgressOpType,
    historyUrl,
    selectedProfileKey,
    isOpInProgress,
    isNewServer,
    isExistingServer
  }
}]

const servers = [ ['site_servers'], serverList => {
  let aclStore = userAclFlux.getAcl();
  let canSsh = aclStore.getSshLogins().size > 0;
  let sshLogins = aclStore.getSshLogins().toJS();

  return serverList.map(srvMap => {
    let role = srvMap.get('role');
    let displayRole = srvMap.get('display_role') || role;
    return {
      canSsh,
      sshLogins,
      publicIp: srvMap.get('public_ipv4'),
      advertiseIp: srvMap.get('advertise_ip'),
      hostname: srvMap.get('hostname'),
      id: srvMap.get('id'),
      instanceType: srvMap.get('instance_type'),
      role,
      displayRole
    }
  }).toJS();
}];

function createProfiles(siteId){
  let profileList = reactor.evaluate(siteGetters.siteProfiles(siteId));
  let profiles = {};
  profileList.
    filter( item => {
      let expandable = item.get('expandPolicy') !== ExpandPolicyEnum.FIXED;
      return expandable;
    }).
    forEach(item => {
      let key = item.get('name');
      let summary = siteGetters.createProfileSummary(item, ProviderEnum.AWS);
      profiles[key] = {
        value: key,
        ...summary
    }
  });

  return profiles;
}

function ensureAllowedInstanceTypes(profileKey, { fixedInstanceType, instanceTypes} ){
  let serverList = reactor.evaluate(['site_servers']);
  let srvMap = serverList.find( srvMap => srvMap.get('role') === profileKey );

  if(!srvMap || !fixedInstanceType){
    return instanceTypes;
  }else{
    return [srvMap.get('instance_type')]
  }
}

export default {
  existingServerPendingOperation,
  existingServer,
  newServer,
  deleteOperationAttempt: requestStatus(TRYING_TO_DELETE_EXPAND_OPERATION),
  startOperationAttempt: requestStatus(TRYING_TO_START_EXPAND_OPERATION),
  createOperationAttempt: requestStatus(TRYING_TO_CREATE_EXPAND_OPERATION),
  serverProvision,
  servers,
  serverToRemove,
  removeServerAttemp: requestStatus(TRYING_TO_START_SHRINK_OPERATION)
}
