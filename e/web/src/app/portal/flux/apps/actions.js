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
