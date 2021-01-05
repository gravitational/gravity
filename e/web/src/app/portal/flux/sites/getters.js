import reactor from 'oss-app/reactor';
import { SiteStateEnum } from 'oss-app/services/enums';
import { TRYING_TO_DELETE_SITE } from 'oss-app/flux/restApi/constants';
import { TRYING_TO_UNLINK_SITE } from './actionTypes';
import { requestStatus } from 'oss-app/flux/restApi/getters';

const siteToUnlink = ['portal_sites', 'siteToUnlink'];

const siteToDelete = [['portal_sites', 'siteToDelete'], siteId => {
  if(!siteId){
    return null;
  }

  let siteMap = reactor.evaluate(['sites', siteId]);
  let provider = siteMap.get('provider');

  return {
    siteId,
    provider
  }
}];

const sitesInfo = [ ['sites'], (sitesMap) => {
    return sitesMap
      .valueSeq()
      .filter( itemMap => itemMap.get('local') !== true)
      .sortBy( itemMap=> itemMap.get('created') )
      .map( itemMap => {
        let appVersion = itemMap.getIn(['app', 'package', 'version']);
        let appName = itemMap.getIn(['app', 'package', 'name']);
        let appDisplayName = itemMap.getIn(['app', 'manifest', 'displayName']);
        let item = itemMap.toJS();
        appDisplayName = appDisplayName || appName;
        return{
          id: item.id,
          labels: createLabel(itemMap.get('labels')),
          provisioner: item.provisioner,
          provider: item.provider,
          domainName: item.domain,
          location: item.location,
          appDisplayName,
          appName,
          appVersion,
          createdBy: itemMap.get('created_by') || 'unknown',
          created: item.created,
          state: createSiteState(item),
          installUrl: item.installerUrl,
          siteUrl: item.siteUrl
        }
    }).toArray();
  }
];

function createLabel(labelMap){
  if(!labelMap){
    return {};
  }

  return labelMap.toJS();
}

function createSiteState(siteData){
  var state = {
    isShrinking: false,
    isUninstalling: false,
    isInstalling: false,
    isExpanding: false,
    isCreated: false,
    isDeployed: false,
    isDegraded: false,
    isOffline: false,
    isUndefined: false
  };

  switch (siteData.state) {
    case SiteStateEnum.INSTALLING:
      state.isInstalling = true;
      break
    case SiteStateEnum.SHRINKING:
      state.isShrinking = true;
      break
    case SiteStateEnum.UNINSTALLING:
      state.isUninstalling = true;
      break
    case SiteStateEnum.EXPANDING:
      state.isExpanding = true;
      break;
    case SiteStateEnum.NOT_INSTALLED:
      state.isCreated = true;
      break;
    case SiteStateEnum.ACTIVE:
      state.isDeployed = true;
      break;
   case SiteStateEnum.FAILED:
      state.isFailed = true;
      break;
    case SiteStateEnum.DEGRADED:
      state.isDegraded = true;
      break;
    case SiteStateEnum.OFFLINE:
      state.isOffline = true;
      break;
    case SiteStateEnum.UPDATING:
      state.isUpdating = true;
      break;
   default:
      state.isUndefined = true;
   }

   return state;
}

export default {
  sitesInfo,
  siteToDelete,
  siteToUnlink,
  unlinkSiteAttemp: requestStatus(TRYING_TO_UNLINK_SITE),
  deleteSiteAttemp: requestStatus(TRYING_TO_DELETE_SITE)
 }
