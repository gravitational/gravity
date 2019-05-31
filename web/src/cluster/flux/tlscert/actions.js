/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import reactor from 'app/reactor';
import cfg from 'app/config';
import api from 'app/services/api';
import Logger from 'app/lib/logger';
import { Uploader } from 'app/services/uploader';
import { SETTINGS_CERT_RECEIVE } from './actionTypes';

const logger = Logger.create('cluster/flux/tlscert/actions');

export function saveTlsCert(siteId, certificate, private_key, intermediate) {
  const data = {
    certificate,
    private_key,
    intermediate
  };

  const upoader = new Uploader(cfg.getSiteTlsCertUrl(siteId));
  return upoader.start(data)
    .done(json => {
      reactor.dispatch(SETTINGS_CERT_RECEIVE, json)
    }).fail(err => {
      logger.error('saveTlsCert()', err);
    })
}

export function fetchTlsCert(siteId) {
  return api.get(cfg.getSiteTlsCertUrl(siteId)).done(json => {
    reactor.dispatch(SETTINGS_CERT_RECEIVE, json)
  })
}