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

import cfg from 'app/config';
import reactor from 'app/reactor';
import siteGetters from 'app/flux/sites/getters';
import opGetters from 'app/flux/operations/getters';
import appsGetters from 'app/flux/apps/getters';
import { requestStatus } from 'app/flux/restApi/getters';
import { RemoteAccessEnum } from 'app/services/enums';
import {
  TRYING_TO_UPDATE_SITE_REMOTE_ACCESS,
  TRYING_TO_START_APP_UPD_OPERATION,
  TRYING_TO_UPDATE_SITE_LICENSE } from 'app/flux/restApi/constants';

const getSiteId = [ ['site_current', 'id'], id => id];

const appToUpdateTo = ['site_current', 'appToUpdateTo'];

const currentSiteNav = [ ['site_current', 'nav'], navMap => navMap.toJS()];

const currentSiteEndpoints = [ ['site_current', 'endpoints'], endpointsList => {
  if(!endpointsList){
    return [];
  }

  return endpointsList.map(itemMap => {
    let { name, description, addresses } = itemMap.toJS();
    addresses = addresses || [];

    let urls = addresses.map( addr => addr);
    return {
      name,
      description,
      urls
    }
  }).toJS();

}];

const currentSiteRemoteAccess = [ ['site_current'], currentSiteMap => {
  return {
    enabled: currentSiteMap.get('remoteAccess') === RemoteAccessEnum.ON,
    isDialogOpen: currentSiteMap.get('remoteAccessDialogOpen')
  }
}];

const currentSite = () => {
  let siteId = reactor.evaluate(getSiteId);
  let siteReportUrl = cfg.getSiteReportUrl(siteId);
  return  [ ['sites', siteId], siteMap => {
    if(!siteMap){
      return null;
    }

    let license = reactor.evaluate(siteGetters.siteLicense(siteId));
    let remoteAccess = reactor.evaluate(['site_current', 'remoteAccess']);

    return {
      siteReportUrl,
      license,
      isRemoteAccessVisible: remoteAccess !== RemoteAccessEnum.NA,
      isRemoteAccessEnabled: remoteAccess === RemoteAccessEnum.ON,
      appInfo: siteGetters.createAppInfo(siteMap.get('app')),
      id: siteMap.get('id'),
      status2: siteMap.get('state2').toJS(),
      domainName: siteMap.get('domain'),
      provider: siteMap.get('provider'),
      location: siteMap.get('location'),
    }
  }];
}

const newVersions = [['sites'], () => {
  let siteId = reactor.evaluate(getSiteId);
  let packageMap = reactor.evaluate(['sites', siteId, 'app', 'package']);
  let {name, repository, version} = packageMap.toJS();
  let appList = appsGetters.findNewVersion({ name, repository, version });
  return appList.map(item => {
    let appId = item.get('id');
    let appInfo = siteGetters.createAppInfo(item);
    let ops = reactor.evaluate(opGetters.findInProgressUpdateOpsByAppId(appId));
    if(ops && ops.length > 0){
      appInfo.opId = ops[0].get('id');
    }

    return appInfo;
  }).toJS();
}];

function getWebConfig(){
  let siteId = reactor.evaluate(getSiteId);
  let siteConfig = reactor.evaluate(siteGetters.webConfig(siteId));
  return siteConfig;
}

export default {
  newVersions,
  getSiteId,
  currentSite,
  currentSiteNav,
  currentSiteEndpoints,
  currentSiteRemoteAccess,
  appToUpdateTo,
  changeRemoteAccessAttemp: requestStatus(TRYING_TO_UPDATE_SITE_REMOTE_ACCESS),
  updateAppAttemp: requestStatus(TRYING_TO_START_APP_UPD_OPERATION),
  updateLicenseAttemp: requestStatus(TRYING_TO_UPDATE_SITE_LICENSE),
  getWebConfig
}
