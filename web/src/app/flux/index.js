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
import notifStore from './notifications/notificationStore';
import opStore from './operations/opStore';
import opProgressStore from './opProgress/opProgressStore';
import opAgentStore from './opAgent/opAgentStore';
import opAgentServerStore from './opAgent/opAgentServerStore';
import userStore from './user/userStore.js';
import restApiStore from './restApi/restApiStore.js';
import siteStore from './sites/siteStore';
import siteDialogStore from './sites/siteDialogStore';
import appStore from './apps/appStore';

import './userAcl';
import './nodes';
import './user/userTokenStore.js';

reactor.registerStores({  
  'notifications': notifStore,
  'op': opStore,
  'opProgress': opProgressStore,
  'opAgent': opAgentStore,
  'opAgentServers': opAgentServerStore,
  'user': userStore,
  'rest_api': restApiStore,
  'sites': siteStore,
  'sitesDialogs': siteDialogStore,
  'apps': appStore
});

export const utils = {
  // temporary helper until we remove accountId from API
  getAccountId(){
    let { accountId } = reactor.evaluate(['user'])
    return accountId;
  }
}
