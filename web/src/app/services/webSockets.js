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

import localStorage from './localStorage';
import cfg from 'app/config';
import {formatPattern} from 'app/lib/patternUtils';
import { utils } from 'app/flux';

const webSockets = {
  
  createLogStreamer(siteId, opId){
    const token = localStorage.getAccessToken();    
    const hostname = cfg.getWsHostName();
    const accountId = utils.getAccountId();
    const url = formatPattern(cfg.api.operationLogsPath, {
        siteId, 
        accountId, 
        token, 
        opId
      });

    return new WebSocket(hostname + url);
  }
}


export default webSockets;
