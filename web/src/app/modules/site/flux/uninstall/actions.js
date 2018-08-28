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
import api from 'app/services/api';
import $ from 'jQuery';
import cfg from 'app/config';
import Logger from 'app/lib/logger';
import restApiActions from 'app/flux/restApi/actions';
import { fetchSites } from 'app/flux/sites/actions';
import { TRYING_TO_INIT_UNINSTALLER } from 'app/flux/restApi/constants';
import { RECIEVE_UNINSTALL_STATUS } from './actionTypes';

const logger = Logger.create('site/flux/uninstall/actions');

export const fetchSiteUninstallStatus = siteId => {    
  return api.get(cfg.getSiteUninstallStatusUrl(siteId))
    .fail(handleError)
    .done( json => {      
      reactor.dispatch(RECIEVE_UNINSTALL_STATUS, json);        
    });    
}

export const initUninstaller = siteId => {
  restApiActions.start(TRYING_TO_INIT_UNINSTALLER);        
  return $.when(
    fetchSites(siteId), 
    fetchSiteUninstallStatus(siteId))
    .fail(handleError)
    .done(() => {              
      restApiActions.success(TRYING_TO_INIT_UNINSTALLER);
    });    
}  

const handleError = err => {
  logger.error('initUninstaller', err);
  let msg = api.getErrorText(err);  
  restApiActions.fail(TRYING_TO_INIT_UNINSTALLER, msg);
}