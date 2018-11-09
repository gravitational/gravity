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

import appGetters from 'app/flux/apps/getters';
import reactor from 'app/reactor';
import {requestStatus} from 'app/flux/restApi/getters';
import { TRYING_TO_INIT_INSTALLER } from 'app/flux/restApi/constants';

const installer = [['installer'], map => map.toJS() ];

const userHints = [
    ['installer', 'step'],
    ['installer_provision', 'isOnPrem'],
    (step, isOnPrem) => ({step, isOnPrem})];

const logoUri = [['installer'], map => {
  let {siteId, name, version, repository} = map.toJS();
  let appId = `${repository}/${name}/${version}`;
  let sitePath =  ['sites', siteId, 'app'];
  let appPath =  ['apps', appId];
  let path = siteId ? sitePath : appPath;

  let appMap = reactor.evaluate(path);

  return appMap ? appGetters.createLogoUri(appMap) : null;
}]

export default {
  installer,
  initInstallerAttempt: requestStatus(TRYING_TO_INIT_INSTALLER),
  userHints,
  logoUri
}
