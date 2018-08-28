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
import { RECEIVE_NODES }  from './actionTypes';
import api from 'app/services/api';
import cfg from 'app/config';
import {showError} from 'app/flux/notifications/actions';
import Logger from 'app/lib/logger';

const logger = Logger.create('flux/nodes');

export function  fetchNodes(sid) {
  return api.get(cfg.getNodesUrl(sid))
  .then(res => res.items || [])
  .done(items  =>{
    reactor.dispatch(RECEIVE_NODES, items);
  }).fail((err)=>{
    showError('Unable to retrieve list of nodes');
    logger.error('fetchNodes', err);
  })
}

