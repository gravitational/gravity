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

import { Store, toImmutable } from 'nuclear-js';
import cfg from 'app/config';
import { ProviderEnum } from 'app/services/enums';
import { StepValueEnum } from './../enums';
import { INSTALLER_INIT, INSTALLER_EULA_ACCEPT, INSTALLER_SET_STEP } from './actionTypes';

// installer config section
const installerCfg = cfg.modules.installer;

const stepOptions = [  
  { value: StepValueEnum.LICENSE, title: 'License' },
  { value: StepValueEnum.NEW_APP, title: 'Location' },
  { value: StepValueEnum.PROVISION, title: 'Capacity' },
  { value: StepValueEnum.PROGRESS, title: 'Installation' } ];

export default Store({
  getInitialState() {
    return toImmutable({
      cfg: installerCfg,
      step: null,
      siteId: null,
      opId: null,
      name: null,
      displayName: null,
      repository: null,
      version: null,
      requiresLicense: false,
      eulaAccepted: false,
      eula: {
        enabled: false,
        content: null
      },
      stepOptions: []
    })
  },

  initialize() {
    this.on(INSTALLER_INIT, init);
    this.on(INSTALLER_SET_STEP, setStep);
    this.on(INSTALLER_EULA_ACCEPT, state => state.set('eulaAccepted', true))
  }
})

function setStep(state, step){
  return state.set('step', step);
}

function init(state, props) {    
  let {
    siteId,
    opId,
    step,
    name,
    displayName,
    repository,
    version,
    customConfig = cfg.modules.installer,
    requiresLicense = false,
    eula
  } = props;
  
  // remove license step if no license needed
  if(!requiresLicense){
    stepOptions.shift()
  }

  // only ONPREM provider is available if in standalone mode
  if(cfg.isStandAlone()){
    customConfig.providers = [ ProviderEnum.ONPREM ];
  }

  return state.mergeIn(['cfg'], customConfig)
              .mergeIn(['eula'], eula)
              .set('siteId', siteId)
              .set('opId', opId)
              .set('step', step)
              .set('name', name)
              .set('displayName', displayName)     
              .set('repository', repository)
              .set('requiresLicense', requiresLicense)
              .set('version', version)
              .set('stepOptions', stepOptions)
}
