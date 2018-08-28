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

import { OpStateEnum } from 'app/services/enums';

function isInitiated(state){
  return state === OpStateEnum.EXPAND_INITIATED;
}

function isFailed(state){
  return state === OpStateEnum.FAILED;
}

function isCompleted(state){
  return state ===  OpStateEnum.COMPLETED;
}

function isInProgress(state){
  switch (state) {
    case OpStateEnum.UPDATE_IN_PROGRESS:
    case OpStateEnum.SHRINK_IN_PROGRESS:
    case OpStateEnum.EXPAND_SETTING_PLAN:
    case OpStateEnum.EXPAND_PLANSET:
    case OpStateEnum.EXPAND_PROVISIONING:
    case OpStateEnum.EXPAND_DEPLOYING:
      return true
    default:
      return false
    }
}

export default {
  isFailed,
  isCompleted,
  isInProgress,
  isInitiated
}
