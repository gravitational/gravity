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
