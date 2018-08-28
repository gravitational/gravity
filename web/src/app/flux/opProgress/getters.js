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
import operationGetters from 'app/flux/operations/getters';

const progressById = opId => [['opProgress', opId], progressMap => {
  if(!progressMap){
    return null;
  }

  let siteId = progressMap.get('site_id');
  let state = progressMap.get('state');
  let opType = reactor.evaluate(operationGetters.opTypeById(opId));

  return {
    siteId,
    opId,
    opType,
    step: progressMap.get('step'),
    isProcessing: state === 'in_progress',
    isCompleted: state === 'completed',
    isError: state === 'failed',
    message: progressMap.get('message'),
    crashReportUrl: progressMap.get('crashReportUrl'),
    siteUrl: progressMap.get('siteUrl')
  }
}];

export default {
  progressById
}
