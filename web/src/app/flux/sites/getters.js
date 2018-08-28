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
import appsGetters from 'app/flux/apps/getters';
import {requestStatus} from 'app/flux/restApi/getters';
import { FETCHING_SITES, TRYING_TO_DELETE_SITE } from 'app/flux/restApi/constants';
import { SiteReasonEnum, ExpandPolicyEnum } from 'app/services/enums';

const siteById = id => [['sites', id], siteMap => {
  return siteMap ? siteMap.toJS() : null;
}];

const siteLogo = id => [['sites', id, 'app'], appMap => {
  return appsGetters.createLogoUri(appMap);
}];

const siteProfiles = id => [['sites', id], siteMap => {    
  return siteMap.getIn('app.manifest.nodeProfiles'.split('.'));
}];

const siteToDelete = [['sitesDialogs', 'siteToDelete'], siteId => {
  if(!siteId){
    return null;
  }

  let siteMap = reactor.evaluate(['sites', siteId]);
  let provider = siteMap.get('provider');

  return {
    siteId,
    provider
  }
}];

const siteStateById = id => ['sites', id, 'state'];

const siteLicense = id => [['sites', id], siteMap => {
   let licenseMap = siteMap.getIn(['license', 'payload']);
   let raw = siteMap.getIn(['license', 'raw']);
   let isLicenseInvalid = siteMap.get('reason') === SiteReasonEnum.INVALID_LICENSE;
   let message = isLicenseInvalid ? 'Invalid license' : '';

    if(!licenseMap){
      return null;
    }

    let expiration = new Date(licenseMap.get('expiration'));

    // remove the signature section and format the date to GSM
    licenseMap = licenseMap
      .delete('signature')
      .set('expiration', expiration.toGMTString());

    return {
      raw,
      info: licenseMap.toJS(),
      status: {
        isActive: !isLicenseInvalid,
        isError: isLicenseInvalid,
        message
      }
    }
  }];

function createProfileSummary(profileMap, provisionerType) {
  let requirementsMap = profileMap.get('requirements');
  let ram = requirementsMap.getIn(['ram', 'min']);
  let cpuCount = requirementsMap.getIn('cpu.min'.split('.'));
  let description = `required - RAM: ${ram}, CPU: Core x ${cpuCount}`;
  let count = 0;
  let fixedInstanceType = profileMap.get('expandPolicy') === ExpandPolicyEnum.FIXED_INSTANCE;
  let title = profileMap.get('description') || profileMap.get('serviceRole');
  let instanceTypes = profileMap.getIn(['providers', provisionerType, 'instanceTypes']);

  instanceTypes = instanceTypes ? instanceTypes.toJS() : [];

  return {
    ram,
    cpuCount,
    description,
    instanceTypes,
    count,
    title,
    fixedInstanceType
  }
}

function createAppInfo(appMap){
  let id = appMap.get('id');
  let pkg = appMap.get('package').toJS();
  let releaseNotes = appMap.getIn(['manifest','releaseNotes']);  
  let displayName = appMap.getIn(['manifest', 'metadata', 'displayName']);  
  let {name, version, repository } = pkg;  
  let logoUri = appsGetters.createLogoUri(appMap);

  displayName = displayName || name;

  return {
    id,
    name,
    displayName,
    version,
    repository,
    releaseNotes,    
    logoUri
  }
}

export default {
  siteById,
  siteStateById,
  siteLogo,
  siteProfiles,
  siteLicense,  
  siteToDelete,
  deleteSiteAttemp: requestStatus(TRYING_TO_DELETE_SITE),
  fetchSitesAttemp: requestStatus(FETCHING_SITES),
  createProfileSummary,
  createAppInfo
}
