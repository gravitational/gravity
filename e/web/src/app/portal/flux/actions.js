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
