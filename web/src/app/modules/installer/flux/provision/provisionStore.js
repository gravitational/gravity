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
import opGetters from 'app/flux/operations/getters';
import sitesGetters from 'app/flux/sites/getters';
import reactor from 'app/reactor';
import { ProvisionerEnum } from 'app/services/enums';
import {
  INSTALLER_PROVISION_INIT,
  INSTALLER_PROVISION_SET_FLAVOR,
  INSTALLER_PROVISION_SET_INSTANCE_TYPE } from './actionTypes';

import Logger from 'app/lib/logger';

const logger = Logger.create('modules/installer/flux/provisionStore');

export default Store({
  getInitialState() {
    return toImmutable({
      siteId: null,
      isOnPrem: false,
      license: null,
      flavors: [],
      flavorsTitle: null,
      flavorsSelector: null,
      profiles: {},
      profilesToProvision: {}      
    });
  },

  initialize() {
    this.on(INSTALLER_PROVISION_INIT, init);
    this.on(INSTALLER_PROVISION_SET_FLAVOR, setFlavorNumber);
    this.on(INSTALLER_PROVISION_SET_INSTANCE_TYPE, setProfileInstance);
  }
})

function init(state, {profiles, isOnPrem = false, siteId, opId, flavors}) {  
  state = state.set('siteId', siteId)
               .set('opId', opId)
               .set('isOnPrem', isOnPrem);

  state = initFlavors(state, flavors);
  state = initProfiles(state, profiles);
  state = updateProfilesToProvision(state);
  return state;
}

function initProfiles(state, profiles){
  let provMap = toImmutable(profiles);
  let opId = state.get('opId');
  let instructionsMap = reactor.evaluate(opGetters.getOnPremInstructions(opId));

  return state.withMutations(state => {
    provMap.forEach(item => {
      let key = item.get('name');
      let instructions = instructionsMap.getIn([key, 'instructions']);
      let serverRole = key;
      let summary = sitesGetters.createProfileSummary(item, ProvisionerEnum.AWS);

      state.setIn(['profiles', key], toImmutable({
        ...summary,
        isAws: true,
        instanceType: null,
        serverRole,
        instructions
      }))
    })
  })
}

function initFlavors(state, json = {}) {    
  let flavorsDefaultName = json.default;
  let flavorsItems = json.items || [];
  let flavorsTitle = json.title || '';

  if(flavorsItems.length === 0 ){
    return state;
  }

  try{
    let current = 0;    
    let options = flavorsItems.map( (item, index) => {
      let { description, name } = item;
      if (flavorsDefaultName === name) {
          current = index;  
      }

      return {
        value: index,
        label: description || name
      }
    });

    let flavorsSelector = toImmutable({
      current,
      options
    });

    return state.set('flavors', toImmutable(flavorsItems))
                .set('flavorsTitle', flavorsTitle)
                .set('flavorsSelector', flavorsSelector);

  }catch(ex){
    logger.error('Failed to initit flavors', ex);
    return state;
  }
}

function setProfileInstance(state, { profileName, instanceType }){
  return state.setIn(['profilesToProvision', profileName, 'instanceType'], instanceType);
}

function setFlavorNumber(state, value){
  state = state.setIn(['flavorsSelector','current'], value);
  return updateProfilesToProvision(state);
}

function updateProfilesToProvision(state) {  
  let current = state.getIn(['flavorsSelector','current']);  
  let flavorProfiles = state.getIn(['flavors', current, 'profiles']);  
  if(flavorProfiles){
    // init new profiles to provision
    state = state.withMutations(state => {
      state.set('profilesToProvision', toImmutable({}));
      flavorProfiles.toKeyedSeq().forEach((item, key)=>{
        let instanceTypes = item.get('instance_types');
        let instanceType = item.get('instance_type');
        let instanceTypeFixed = !!instanceType;
        let count = item.get('count');
        let profileMap = state.getIn(['profiles', key]);

        if(!instanceType){
          instanceType = item.getIn(['instance_types', 0]);
        }

        profileMap = profileMap.merge({
          count,
          instanceTypes,
          instanceType,
          instanceTypeFixed
        });

        state.setIn(['profilesToProvision', key], profileMap);
      });
    });
  }

  return state;
}