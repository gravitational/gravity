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

import {requestStatus} from 'app/flux/restApi/getters';
import { UserTokenTypeEnum } from 'app/services/enums';

import {
  FETCHING_USER_TOKEN,
  TRYING_TO_COMPLETE_USER_TOKEN,
  TRYING_TO_LOGIN,  
  TRYING_TO_CHANGE_PSW
   } from 'app/flux/restApi/constants';

const user = ['user'];

const userRequestInfo =  [ ['userTokens'], info => {
  return {
    token: info.get('token'),
    qrCode: info.get('qr_code'),
    userName: info.get('user'),
    accountId: info.get('account_id'),    
    isInvite: info.get('type') === UserTokenTypeEnum.INVITE,    
    isReset: info.get('type') === UserTokenTypeEnum.RESET
  }
}];

export default {
  user,    
  userRequestInfo,
  completeUserTokenAttemp: requestStatus(TRYING_TO_COMPLETE_USER_TOKEN),
  fetchUserTokenAttemp: requestStatus(FETCHING_USER_TOKEN),
  loginAttemp: requestStatus(TRYING_TO_LOGIN),    
  pswChangeAttemp: requestStatus(TRYING_TO_CHANGE_PSW)
}
