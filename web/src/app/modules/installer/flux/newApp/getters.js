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

import { toImmutable } from 'nuclear-js';
import {requestStatus} from 'app/flux/restApi/getters';
import {ProviderEnum} from 'app/services/enums';
import {CREATE_NEW_VPC_OPTION_VALUE} from './constants';

import {
  TRYING_TO_VALIDATE_DOMAIN_NAME,
  TRYING_TO_CREATE_NEW_SITE,
  TRYING_TO_VALIDATE_LICENSE,
  TRYING_TO_VERIFY_PROVIDER_KEYS } from 'app/flux/restApi/constants';

const newApp = [
  ['installer_new_app'],
  ['installer', 'cfg', 'enableTags'],
  (newAppMap, enableTags) => {
    return {
      ...newAppMap.toJS(),
        enableTags
    }
  }
]

const onpremProvider = [['installer_new_app', 'providers', ProviderEnum.ONPREM], onPremProviderMap => {
  return onPremProviderMap.toJS();
}]

const awsProvider = [['installer_new_app', 'providers', ProviderEnum.AWS], awsProviderMap => {
  let { access_key, secret_key, session_token, verified } = awsProviderMap.get('keys').toJS();
  let regionMap = getSelectedRegion(awsProviderMap);

  return {
    access_key,
    secret_key,
    session_token,
    keysVerified: verified,
    name: awsProviderMap.get('name'),
    useExisting: awsProviderMap.get('useExisting'),
    selectedVpc: awsProviderMap.get('selectedVpc'),
    selectedRegion: awsProviderMap.get('selectedRegion'),
    selectedKeyPairName: awsProviderMap.get('selectedKeyPairName'),
    regions: getAwsRegions(awsProviderMap),
    regionVpcs: getRegionVpcs(regionMap),
    regionKeyPairNames: getRegionKeyPairNames(regionMap)
  }
}];

function getSelectedRegion(awsProviderMap){
  let regionName = awsProviderMap.get('selectedRegion');
  let regionMap = awsProviderMap.get('regions')
    .find(item=> item.get('name') === regionName)

  return regionMap;
}

function getFirstAvailableKeyPairName(awsProviderMap){
  let regionMap = getSelectedRegion(awsProviderMap);
  let options = getRegionKeyPairNames(regionMap);

  if(!options[0]){
    return null;
  }

  return options[0].value;
}

function getRegionKeyPairNames(regionMap = toImmutable({})){
  let pairs = regionMap.get('key_pairs');
  if(pairs){
    return pairs.map(item=> ({
        value: item.get('name'),
        label: item.get('name')})
      )
      .toArray();
  }

  return [];
}

function getRegionVpcs(regionMap = toImmutable({})){
  let options = [];
  let vpcs = regionMap.get('vpcs');
  if(vpcs){
    options = vpcs.map(createRegionVpcOption).toArray();
  }

  options.unshift({
    value: CREATE_NEW_VPC_OPTION_VALUE,
    label: 'Create new'
  });

  return options;
}

function createRegionVpcOption(vpcMap){
  let isDefault = vpcMap.get('is_default');
  let value = vpcMap.get('vpc_id');
  let displayName = vpcMap.getIn(['tags', 'Name']);
  let label = value;

  if (isDefault) {
    label = `${value} (default)`
  }

  if (displayName) {
    label = `${label} | ${displayName}`
  }

  return {
    value,
    label
  }
}

function getAwsRegions(awsMap){
  let regions = awsMap.get('regions') || [];
  return regions.map(item => (
    {
      value: item.get('name'),
      label: item.get('name') }
    )).toJS();
}

export default {
  getFirstAvailableKeyPairName,
  newApp,
  awsProvider,
  onpremProvider,
  isDomainNameValid: [['installer_new_app', 'isDomainNameValid'], isValid => isValid],
  validateDomainNameAttempt: requestStatus(TRYING_TO_VALIDATE_DOMAIN_NAME),
  createSiteAttemp: requestStatus(TRYING_TO_CREATE_NEW_SITE),
  verifyKeysAttemp: requestStatus(TRYING_TO_VERIFY_PROVIDER_KEYS),
  verifyLicenseAttemp: requestStatus(TRYING_TO_VALIDATE_LICENSE)
}
