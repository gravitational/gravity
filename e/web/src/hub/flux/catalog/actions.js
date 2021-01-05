import reactor from 'app/reactor';
import service from 'oss-app/services/applications';
import * as actionTypes from './actionTypes';

export function fetchApps() {
  return service.fetchApplications().then(apps => {
    reactor.dispatch(actionTypes.CATALOG_RECEIVE_APPS, apps);
  })
}

