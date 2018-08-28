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
import { SITES_RECEIVE_DATA, SITES_APPLY_CONFIG } from './actionTypes';
import { SiteStateEnum } from 'app/services/enums';
import cfg from 'app/config';
import { parseWebConfig } from 'app/lib/paramUtils';

export default Store({
  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(SITES_RECEIVE_DATA, receiveSites);
    this.on(SITES_APPLY_CONFIG, applySiteConfig);
  }
})

function applySiteConfig(state, siteId){
  let manifestMap = state.getIn([siteId, 'app', 'manifest']);
  let appConfig = manifestMap.get('webConfig') || toImmutable({});
  let extensionsMap = manifestMap.get('extensions') || toImmutable({});  
  let monitoringEnabled = !extensionsMap.getIn(['monitoring', 'disabled']);
  let k8sEnabled = !extensionsMap.getIn(['kubernetes', 'disabled']);
  let configMapsEnabled = !extensionsMap.getIn(['configuration', 'disabled']);
  let logsEnabled = !extensionsMap.getIn(['logs', 'disabled']);
    
  cfg.init(appConfig.toJS());    
  cfg.enableSiteMonitoring(monitoringEnabled);
  cfg.enableSiteK8s(k8sEnabled);
  cfg.enableSiteConfigMaps(configMapsEnabled);
  cfg.enableSiteLogs(logsEnabled);
    
  cfg.enableSettingsLogs(logsEnabled);
  cfg.enableSettingsMonitoring(monitoringEnabled);      
  return state;    
}

function receiveSites(state, siteArray){
  return toImmutable({}).withMutations(state => {
    siteArray.forEach((item) => {
      let itemMap = toImmutable(item);
      let id = itemMap.get('domain');
            
      let webConfigJsonStr = itemMap.getIn(['app', 'manifest', 'webConfig']);
      let webConfig = parseWebConfig(webConfigJsonStr);
            
      itemMap = itemMap
        .setIn(['app', 'manifest', 'webConfig'], toImmutable(webConfig))  
        .set('state2', calcSiteState(item.state))
        .set('id', id)
        .set('created', new Date(item.created))
        .set('installerUrl', cfg.getSiteInstallerRoute(id))
        .set('siteUrl', cfg.getSiteRoute(id))

      state.set(id, itemMap);
    });
  });
}

function calcSiteState(siteState){
  var state = {
    isError: false,
    isReady: false,
    isProcessing: false
  };

  switch (siteState) {
    case SiteStateEnum.ACTIVE:
      state.isReady = true;
      break;
    case SiteStateEnum.EXPANDING:
    case SiteStateEnum.SHRINKING:
    case SiteStateEnum.UPDATING:
    case SiteStateEnum.UNINSTALLING:
      state.isProcessing = true;
      break;
    case SiteStateEnum.FAILED:
    case SiteStateEnum.DEGRADED:
      state.isError = true;
      break;
    default:
   }

   return toImmutable(state);
}
