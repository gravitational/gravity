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

import reactor from 'oss-app/reactor';
import { showSuccess, showError } from 'oss-app/flux/notifications/actions';
import * as siteActions from 'oss-app/flux/sites/actions';
import restApiActions from 'oss-app/flux/restApi/actions';
import {
  TRYING_TO_UNLINK_SITE,
  PORTAL_SITE_OPEN_CONFIRM_UNLINK,
  PORTAL_SITE_CLOSE_CONFIRM_UNLINK
} from './actionTypes';

export function openSiteConfirmUnlink(siteId){
  reactor.dispatch(PORTAL_SITE_OPEN_CONFIRM_UNLINK, siteId)
}

export function closeSiteConfirmUnlink(){
  reactor.dispatch(PORTAL_SITE_CLOSE_CONFIRM_UNLINK);
  restApiActions.clear(TRYING_TO_UNLINK_SITE);
}

export function deleteSite(...props) {
  siteActions.uninstallSite(...props).done(() =>
    showSuccess('', 'Uninstall operation has been started')
  );
}

export function unlinkSite(siteId){
  var data = {
    remove_only: true
  }

  restApiActions.start(TRYING_TO_UNLINK_SITE);
  siteActions.uninstallAndDeleteSite(siteId, data)
    .done(()=>{
      showSuccess('', `${siteId} has been removed`);
      closeSiteConfirmUnlink();
      restApiActions.success(TRYING_TO_UNLINK_SITE);
    })
    .fail(err => {
      let msg = err.responseJSON ? err.responseJSON.message : 'Unknown error';
      showError(msg, `Failed to remove  ${siteId} cluster`);
      restApiActions.fail(TRYING_TO_UNLINK_SITE, msg);
    });
}

export function fetchSites(){
  return siteActions.fetchSites().done(()=>{
    // code here
  })
}
