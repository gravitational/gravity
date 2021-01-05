import reactor from 'oss-app/reactor';
import {requestStatus} from 'oss-app/flux/restApi/getters';
import { TRYING_TO_DELETE_APP, TRYING_TO_CREATE_INSTALL_LINK } from './actionTypes';

const appToDelete = ['portal_apps', 'appToDelete'];

const siteToDelete = [['portal_apps', 'siteToDelete'], siteId => {
  if(!siteId){
    return null;
  }

  let siteMap = reactor.evaluate(['sites', siteId]);
  let appName = siteMap.getIn(['app', 'package', 'name']);
  let provider = siteMap.get('provider');

  return {
    siteId,
    appName,
    provider
  }

}];

export default {
  appToDelete,
  siteToDelete,
  deleteAppAttemp: requestStatus(TRYING_TO_DELETE_APP),
  createInstallLinkAttemp: requestStatus(TRYING_TO_CREATE_INSTALL_LINK)
}
