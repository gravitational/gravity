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

import { requestStatus } from 'app/flux/restApi/getters';
import * as AT from 'app/flux/restApi/constants';

export default {  
  userStore: [['settings_usrs'], users => users.toJS() ],  
  deleteUserAttempt: requestStatus(AT.TRYING_TO_DELETE_USER),
  saveUserAttempt: requestStatus(AT.TRYING_TO_UPDATE_USER),
  inviteUserAttempt: requestStatus(AT.TRYING_TO_INVITE),
  resetUserAttempt: requestStatus(AT.TRYING_TO_RESET_USER)
}
