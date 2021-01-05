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
import cfg from './../../../config';
import api from 'oss-app/services/api';
import * as appsActions from 'oss-app/flux/apps/actions';
import restApiActions from 'oss-app/flux/restApi/actions';
import { showSuccess, showError } from 'oss-app/flux/notifications/actions';
import {
  TRYING_TO_DELETE_APP,
  TRYING_TO_CREATE_INSTALL_LINK,
  PORTAL_APPS_OPEN_CONFIRM_DELETE,
  PORTAL_APPS_CLOSE_CONFIRM_DELETE } from './actionTypes';

export function createOneTimeInstallLink(appId){
  reactor.batch(()=>{
    restApiActions.start(TRYING_TO_CREATE_INSTALL_LINK);
    const [ repository, name, version ] = appId.split('/');
    const packageLocator = `${repository}/${name}:${version}`
    return api.post(cfg.api.oneTimeInstallLinkPath, {app: packageLocator})
      .then(res => {
        let { token } = res;
        let oneTimeinstallUrl = cfg.getInstallNewSiteOneTimeLinkRoute(name, repository, version, token);
        return oneTimeinstallUrl;
      })
      .done(url => {
        restApiActions.success(TRYING_TO_CREATE_INSTALL_LINK, url);
      })
      .fail(()=>{
        restApiActions.fail(TRYING_TO_CREATE_INSTALL_LINK);
      })
  });
}

export function openAppConfirmDelete(appId){
  reactor.dispatch(PORTAL_APPS_OPEN_CONFIRM_DELETE, appId)
}

export function closeAppConfirmDelete(){
  reactor.dispatch(PORTAL_APPS_CLOSE_CONFIRM_DELETE);
  restApiActions.clear(TRYING_TO_DELETE_APP);
}

export function deleteApp(appId){
  const [, name] = appId.split('/');
  restApiActions.start(TRYING_TO_DELETE_APP);
  appsActions.deleteApp(appId)
    .then(() => appsActions.fetchApps())
    .done(() => {
        showSuccess('', `${name} has been deleted`);
        restApiActions.success(TRYING_TO_DELETE_APP);
        closeAppConfirmDelete();
      })
    .fail(err => {
      let msg = err.responseJSON ? err.responseJSON.message : 'Unknown error';
      showError(msg, `Failed to delete the ${name}`);
      restApiActions.fail(TRYING_TO_DELETE_APP, msg);
    })
}
