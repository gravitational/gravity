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
import api from 'app/services/api';
import cfg from 'app/config';
import {
  OP_AGENT_RECEIVE,
  OP_AGENT_SERVERS_SET_VARS,
  OP_AGENT_SERVERS_CLEAR }  from './actionTypes';

const actions = {
  
  clearAgentServerVars(serverRole){
    reactor.batch(()=>{
      reactor.dispatch(OP_AGENT_SERVERS_CLEAR, serverRole);
    });
  },

  setAgentServerVars(serverRole, serverConfigs){
    reactor.dispatch(OP_AGENT_SERVERS_SET_VARS, { serverRole, serverConfigs } );
  },

  fetchReport(siteId, opId){
    return api.get(cfg.getOperationAgentUrl(siteId, opId)).then(data => {
      reactor.dispatch(OP_AGENT_RECEIVE, {opId, report: data});
    });
  }
}

export default actions;