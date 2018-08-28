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
import { Store, toImmutable } from 'nuclear-js';
import  { USER_CONFIRM_RECEIVE_DATA } from './actionTypes';

const store = Store({
  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(USER_CONFIRM_RECEIVE_DATA, receiveConfirmInfo);
  }
})

function receiveConfirmInfo(state, confirmInfo){  
  return toImmutable(confirmInfo);
}

const STORE_NAME = 'userTokens';

reactor.registerStores({ [STORE_NAME] : store });