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

import api from 'app/services/api';
import cfg from 'app/config';
import reactor from 'app/reactor';
import {showError} from 'app/flux/notifications/actions';
import restApiActions from 'app/flux/restApi/actions';
import { TRYING_TO_UPDATE_RETENTION_VALUES } from 'app/flux/restApi/constants';
import { RetentionValueEnum } from 'app/services/enums';
import getters from './getters';
import { RECEIVE_VALUES } from './actionTypes';

const createRetentionValue = (name, duration) => ( { name, duration: duration } )

export function fetchRetentionValues(siteId){  
  return api.get(cfg.getSiteMonitorRetentionValuesUrl(siteId))
    .done(json => {
      json = json || [];
      let defVal, medVal, longVal;

      json.forEach( o => {
        switch (o.name) {
          case RetentionValueEnum.DEF:
            defVal = o.duration;  
            break;
          case RetentionValueEnum.MED:
            medVal = o.duration;  
            break;
          case RetentionValueEnum.LONG:
            longVal = o.duration;  
            break;          
          default:
            break;
        }
      });

      reactor.dispatch(RECEIVE_VALUES, { defVal, medVal, longVal })
    })
    .fail( err =>{
      let msg = api.getErrorText(err);
      showError(msg, 'Cannot retrieve retention values');
    });
}

export function saveRetentionValues(defVal, medVal, longVal){
  let siteId = reactor.evaluate(getters.siteId);
  let data = [
    createRetentionValue(RetentionValueEnum.DEF, defVal),
    createRetentionValue(RetentionValueEnum.MED, medVal),
    createRetentionValue(RetentionValueEnum.LONG, longVal),
  ];
  
  restApiActions.start(TRYING_TO_UPDATE_RETENTION_VALUES);    
  api.put(cfg.getSiteMonitorRetentionValuesUrl(siteId), data)
    .done(json => {        
      restApiActions.success(TRYING_TO_UPDATE_RETENTION_VALUES);
      reactor.dispatch(RECEIVE_VALUES, { defVal, medVal, longVal })
      reactor.dispatch(TRYING_TO_UPDATE_RETENTION_VALUES, json)
    })
    .fail(err => {
      let msg = api.getErrorText(err);                
      showError(msg, 'Failed to update retention values');
      restApiActions.fail(TRYING_TO_UPDATE_RETENTION_VALUES);
    })
}
