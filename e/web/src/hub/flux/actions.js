import Logger from 'oss-app/lib/logger';
import { Activator } from 'oss-app/lib/featureBase';
import { fetchUserContext } from 'oss-app/flux/user/actions';
import * as featureFlags from 'oss-app/cluster/featureFlags';
import service, { applyConfig } from 'oss-app/services/clusters';
import cfg from 'app/config';
import { setClusters, updateClusters } from './clusters/actions';
import { setCluster } from 'oss-app/flux/cluster/actions';

const logger = Logger.create('hub/flux/actions');

export function initHub(features) {
  const siteId = cfg.getLocalSiteId();
  cfg.setDefaultSiteId(siteId);
  return fetchUserContext()
    .then(() => init(features))
    .fail(err => {
      logger.error('initHub()', err);
    });
}

function init(features){
  return service.fetchCluster({ shallow: false })
    .then(cluster => {
      setCluster(cluster);
      // Apply cluster web config settings
      applyConfig(cluster);
      // Init features
      const activator = new Activator(features);
      activator.onload({ featureFlags });
    })
}

export function fetchClusters(){
  return service.fetchClusters({shallow: false}).then(clusters => {
    setClusters(clusters);
  })
}

export function refreshClusters(){
  return service.fetchClusters().then(clusters => {
    updateClusters(clusters);
  })
}

export function unlinkCluster(siteId){
  // unlink the cluster and then re-fetch to update
  return service.unlink(siteId).then(() => fetchClusters());
}