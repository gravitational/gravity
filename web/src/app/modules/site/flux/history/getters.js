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
import opGetters from 'app/flux/operations/getters';
import helpers from 'app/flux/operations/helpers';
import {displayDate} from 'app/lib/dateUtils';
import {OpTypeEnum} from 'app/services/enums';
import cfg from 'app/config';

const siteHistory = [
  ['site_current', 'id'],
  ['site_history'], (siteId, siteHistoryMap) => {
    let {isInitialized, selectedOpId} = siteHistoryMap.toJS();
    return {
      isInitialized,
      selectedOpId,
      siteId
    }
  }];

const siteOps = [
  ['site_current', 'id'],
  ['site_history', 'selectedOpId'],
  ['op'],
  (siteId, selectedOpId) =>{
    var ops = reactor.evaluate(opGetters.opsBySiteId(siteId));

    return ops.map(item=>{
      let state = item.get('state');
      let opId = item.get('id');
      let logsUrl = cfg.getSiteLogQueryRoute(siteId, `file:${opId}`);
      let type = item.get('type');
      let displayType = getTitle(item);

      return {
        opId: opId,
        isSelected: opId === selectedOpId,
        siteId: item.get('site_id'),
        type,
        displayType,
        created: displayDate(item.get('created')),
        description: getDescription(item),
        isCompleted: helpers.isCompleted(state),
        isInitiated: helpers.isInitiated(state),
        isFailed: helpers.isFailed(state),        
        isProcessing: helpers.isInProgress(state),
        logsUrl
      };
    });
  }
];

function getTitle(opMap){
  switch (opMap.get('type')) {
    case OpTypeEnum.OPERATION_UPDATE:
      let app = opMap.getIn(['data', 'update_package']) || '';
      let [, ver] = app.split(':');
      return `Updating to ${ver}`;
    case OpTypeEnum.OPERATION_INSTALL:
      return 'Installing this cluster';
    case OpTypeEnum.OPERATION_EXPAND:
      return 'Adding a server';
    case OpTypeEnum.OPERATION_SHRINK:
      return 'Removing a server';
    case OpTypeEnum.OPERATION_UNINSTALL:
      return 'Uninstalling this cluster';  
    default:
      return `Unknown`;
  }
}

function getDescription(op){
  var profiles = op.getIn(['data', 'settings', 'profiles']);
  return profiles ? profiles.toSeq().first().get('description') : '';
}

export default {
  siteOps,
  siteHistory
}
