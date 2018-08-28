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
import { Store, toImmutable } from 'nuclear-js';
import { ProviderEnum } from 'app/services/enums';
import getters from './getters';

const defaultServiceSubnet = '10.100.0.0/16';
const defaultPodSubnet = '10.244.0.0/16';

import  {
  INSTALLER_NEW_APP_INIT,
  INSTALLER_NEW_APP_SET_PROVIDER,
  INSTALLER_NEW_APP_SET_DOMAIN_NAME,
  INSTALLER_NEW_APP_SET_DOMAIN_NAME_VALID_FLAG,
  INSTALLER_NEW_APP_SET_USE_EXISTING,
  INSTALLER_NEW_APP_SET_PROVIDER_KEYS,
  INSTALLER_NEW_APP_RECEIVE_PROVIDER_DATA,
  INSTALLER_NEW_APP_SET_AWS_REGION,
  INSTALLER_NEW_APP_SET_AWS_VPC,
  INSTALLER_NEW_APP_SET_TAGS,
  INSTALLER_NEW_APP_SET_KEY_PAIR_NAME,
  INSTALLER_NEW_APP_SET_LICENSE,
  INSTALLER_NEW_APP_SET_PROVIDER_KEYS_VARIFIED,
  INSTALLER_NEW_APP_SET_ONPREM_CIDRS
} from './actionTypes';

export default Store({
  getInitialState() {
    return toImmutable({
      domainName: '',
      isDomainNameValid: false,
      selectedProvider: null,
      licenseInfo: {},
      license: null,
      tags: {},
      availableProviders: [],
      providers: {}
    })
  },

  initialize() {
    this.on(INSTALLER_NEW_APP_INIT, init);
    this.on(INSTALLER_NEW_APP_SET_PROVIDER, setCurrentProvider);
    this.on(INSTALLER_NEW_APP_SET_DOMAIN_NAME, setDomainName);
    this.on(INSTALLER_NEW_APP_SET_DOMAIN_NAME_VALID_FLAG, setDomainNameIsValidFlag);
    this.on(INSTALLER_NEW_APP_SET_USE_EXISTING, setNewOrExistingServers);
    this.on(INSTALLER_NEW_APP_SET_PROVIDER_KEYS, setProviderKeys);
    this.on(INSTALLER_NEW_APP_SET_PROVIDER_KEYS_VARIFIED, setProviderKeysVerified);
    this.on(INSTALLER_NEW_APP_RECEIVE_PROVIDER_DATA, receiveProviderData);
    this.on(INSTALLER_NEW_APP_SET_AWS_REGION, setAwsRegion);
    this.on(INSTALLER_NEW_APP_SET_AWS_VPC, setAwsVpc);
    this.on(INSTALLER_NEW_APP_SET_TAGS, setAppTags);
    this.on(INSTALLER_NEW_APP_SET_KEY_PAIR_NAME, setProviderKeyPairName);
    this.on(INSTALLER_NEW_APP_SET_LICENSE, setLicense);
    this.on(INSTALLER_NEW_APP_SET_ONPREM_CIDRS, setOnpremSubnets)
  }
})

function init(state, data){
  let providers = getProviderSettings();
  let availableProviders = getAvailableProviders();

  return state.merge(data)
    .set('providers', providers)
    .set('availableProviders', availableProviders);
}

function setLicense(state, license){
  return state.set('license', license);
}

function setAppTags(state, tags){
  return state.set('tags', toImmutable(tags));
}

function setOnpremSubnets(state, subnets) {
  let curProvider = state.get('selectedProvider');
  return state.mergeIn(['providers', curProvider], subnets);
}

function setAwsVpc(state, value){
  let curProvider = state.get('selectedProvider');
  return state.setIn(['providers', curProvider, 'selectedVpc'], value);
}

function setAwsRegion(state, value){
  let curProvider = state.get('selectedProvider');
  let provider = state.getIn(['providers', curProvider]);

  provider = provider.set('selectedRegion', value)
                     .set('selectedVpc', null);

  let selectedKeyPairName = getters.getFirstAvailableKeyPairName(provider);

  provider = provider.set('selectedKeyPairName', selectedKeyPairName);

  return state.setIn(['providers', curProvider], provider);
}

function receiveProviderData(state, data){
  let curProvider = state.get('selectedProvider');
  return state.mergeIn(['providers', curProvider], data[curProvider]);
}

function setProviderKeysVerified(state, value){
  let curProviderName = state.get('selectedProvider');
  return state.setIn(['providers', curProviderName, 'keys', 'verified' ], value);
}

function setProviderKeys(state, {access_key=null, secret_key=null, session_token=null}){
  let curProviderName = state.get('selectedProvider');
  return state.mergeIn(['providers', curProviderName, 'keys' ], {access_key, secret_key, session_token, verified: false});
}

function setDomainNameIsValidFlag(state, value){
  return state.set('isDomainNameValid', value);
}

function setNewOrExistingServers(state, useExisting){
  let curProviderName = state.get('selectedProvider');
  return state.setIn(['providers', curProviderName, 'useExisting'], useExisting);
}

function setProviderKeyPairName(state, keyPairName){
  let curProviderName = state.get('selectedProvider');
  return state.setIn(['providers', curProviderName, 'selectedKeyPairName'], keyPairName);
}

function setDomainName(state, name){
  return state.set('domainName', name)
              .setIn(['tags', 'Name'], name)
              .set('isDomainNameValid', false);
}

function setCurrentProvider(state, name){
  return state.set('providers', getProviderSettings())
              .set('selectedProvider', name);
}


function getAvailableProviders(){
  return reactor.evaluate(['installer','cfg', 'providers']);
}

function getProviderSettings(){
  let providersMap = reactor.evaluate(['installer','cfg', 'providerSettings']);
  if(providersMap.has(ProviderEnum.AWS)){
    providersMap = providersMap.mergeIn([ProviderEnum.AWS], {
      keys: {
        verified: false,
        access_key: null,
        secret_key: null,
        session_token: null,
        keyPairName: null
      },
      vpcs: [],
      regions: []
    });
  }

  providersMap = providersMap.mergeIn([ProviderEnum.ONPREM], {
    serviceSubnet: defaultServiceSubnet,
    podSubnet: defaultPodSubnet
  });

  return providersMap;
}
