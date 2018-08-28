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
import siteGetters from 'app/flux/sites/getters';
import opGetters from 'app/flux/operations/getters';
import opAgentActions from 'app/flux/opAgent/actions';
import opAgentGetters from 'app/flux/opAgent/getters';
import installerGetters from './../installer/getters';
import getters from './getters';
import api from 'app/services/api';
import cfg from 'app/config';
import _ from 'lodash';

import {
  INSTALLER_PROVISION_INIT,
  INSTALLER_PROVISION_SET_FLAVOR,
  INSTALLER_PROVISION_SET_INSTANCE_TYPE } from './actionTypes';

import { TRYING_TO_START_INSTALL } from 'app/flux/restApi/constants';
import restApiActions from 'app/flux/restApi/actions';

export function fetchAgentReport(){
    let {siteId, opId} = reactor.evaluate(installerGetters.installer);
    return opAgentActions.fetchReport(siteId, opId);
  }

export function setAwsInstanceType(profileName, instanceType){
  reactor.dispatch(INSTALLER_PROVISION_SET_INSTANCE_TYPE, {
    profileName,
    instanceType
  })
}

export function startInstallPrecheck() {
  startInstall(true);
}

export function startInstall(precheckOnly){
  let {opId, siteId} = reactor.evaluate(installerGetters.installer);
  let {profilesToProvision, isOnPrem, license} = reactor.evaluate(getters.provision);
  let data = {
    license,
    profiles: {}
  };

  if(isOnPrem){
    data.servers = reactor.evaluate(opAgentGetters.serverConfigs);
  }

  _.forIn(profilesToProvision, (value, key) => {
    data.profiles[key] = {
      instance_type: value.instanceType,
      count: value.count
    }
  });
                
  let url = precheckOnly ? 
    cfg.operationPrecheckPath(siteId, opId) : 
    cfg.getOperationStartUrl(siteId, opId);
  
  restApiActions.start(TRYING_TO_START_INSTALL);
  return api.post(url, data)
    .done(()=>{
      // in case of precheck, just notify about success response 
      if(precheckOnly){
        restApiActions.success(TRYING_TO_START_INSTALL);
      }else{
        // reload the page and let the installer to go to the next step          
        window.location.reload();          
      }        
    })
    .fail( err => {
      let msg = api.getErrorText(err);
      restApiActions.fail(TRYING_TO_START_INSTALL, msg);
    })
}

export function setFlavorNumber(value){
  reactor.dispatch(INSTALLER_PROVISION_SET_FLAVOR, value);
}

export function initProvision(siteId, opId, flavors){
  let isOnPrem = reactor.evaluate(opGetters.isOnPrem(opId));
  let profiles = reactor.evaluate(siteGetters.siteProfiles(siteId));
  reactor.dispatch(INSTALLER_PROVISION_INIT,
    {
      siteId,
      opId,
      isOnPrem,
      flavors,
      profiles
    });
}