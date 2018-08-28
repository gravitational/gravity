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
import {requestStatus} from  'app/flux/restApi/getters';
import installerGetters from './../installer/getters';
import { TRYING_TO_START_INSTALL, } from 'app/flux/restApi/constants';
import opAgentGetters from 'app/flux/opAgent/getters';

const provision = [['installer_provision'], map => map.toJS() ];

const onPremServerCount = () => {
  let {opId}  = reactor.evaluate(installerGetters.installer);
  return opAgentGetters.serverCountByOp(opId);
}

export default {
  provision,
  onPremServerCount,
  startInstallAttempt: requestStatus(TRYING_TO_START_INSTALL)
}
