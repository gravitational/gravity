/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import $ from 'jQuery';
import Logger from 'oss-app/lib/logger';
import { fetchApps } from 'oss-app/flux/apps/actions';
import { fetchSites, applyConfig } from 'oss-app/flux/sites/actions';
import restApiActions from 'oss-app/flux/restApi/actions';
import api from 'oss-app/services/api';
import * as userAclFlux from 'oss-app/flux/userAcl';

import { TRYING_TO_INIT_PORTAL } from './actionTypes';
import cfg from './../../config';

const logger = Logger.create('portal/flux/actions');

export function initPortal(){
  restApiActions.start(TRYING_TO_INIT_PORTAL);
  let siteId = cfg.getLocalSiteId();
  let promises = [fetchApps(), fetchSites(siteId)];

  if (userAclFlux.getAcl().getClusterAccess().list) {
    promises.push(fetchSites());
  }

  $.when(...promises)
    .then(()=>{
      try{
        applyConfig(siteId);
        restApiActions.success(TRYING_TO_INIT_PORTAL);
      }catch(err){
        return $.Deferred().reject(err);
      }
    })
    .fail(err => {
      logger.error('init', err);
      let msg = api.getErrorText(err);
      restApiActions.fail(TRYING_TO_INIT_PORTAL, msg);
    });
}
