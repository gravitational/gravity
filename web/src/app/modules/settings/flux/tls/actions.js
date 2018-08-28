/*
Copyright 2018 Gravitational, Inc.

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
import { Uploader } from 'app/services/uploader';
import restApiActions from 'app/flux/restApi/actions';
import { TRYING_SAVE_TLS_CERT } from 'app/flux/restApi/constants';
import { SETTINGS_CERT_RECEIVE } from './actionTypes';
import { showError, showSuccess } from 'app/flux/notifications/actions';

export function saveTlsCert(siteId, certificate, private_key, intermediate) {
  let data = {
    certificate,
    private_key,
    intermediate
  };
  
  restApiActions.start(TRYING_SAVE_TLS_CERT);
  let upoader = new Uploader(cfg.getSiteTlsCertUrl(siteId));    
  return upoader.start(data).done(json => {
    restApiActions.success(TRYING_SAVE_TLS_CERT);
    reactor.dispatch(SETTINGS_CERT_RECEIVE, json)    
    showSuccess('Certificate has been updated', '');
  }).fail(err => {
    let msg = api.getErrorText(err);
    showError(msg, '');
  })
}

export function fetchTlsCert(siteId) {
  return api.get(cfg.getSiteTlsCertUrl(siteId)).done(json => {    
    reactor.dispatch(SETTINGS_CERT_RECEIVE, json)
  })
}