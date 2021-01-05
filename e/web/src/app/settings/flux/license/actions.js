import restApiActions from 'oss-app/flux/restApi/actions';
import api from 'oss-app/services/api';
import cfg from 'oss-app/config';
import Logger from 'oss-app/lib/logger';
import { TRYING_TO_CREATE_LICENSE } from './actionTypes';

const logger = Logger.create('settings/flux/actions');

export function initLicenseGen() {
  restApiActions.clear(TRYING_TO_CREATE_LICENSE);
}

export function createLicense(max_nodes, expiration, stop_app) {
  restApiActions.start(TRYING_TO_CREATE_LICENSE);
  const data = {
    max_nodes,
    expiration,
    stop_app
  }

  api.post(cfg.api.licenseGeneratorPath, data)
    .done(res => {
      restApiActions.success(TRYING_TO_CREATE_LICENSE, res.license);
    })
    .fail(err => {
      logger.error('create license', err);
      const msg = api.getErrorText(err);
      restApiActions.fail(TRYING_TO_CREATE_LICENSE, msg);
    });
}
