/**
 * Copyright 2021 Gravitational Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import ServerVersion from 'oss-app/components/common/serverVersion';
import Apps from './apps/main';
import Sites from './sites/main';

const AppsAndSites = () => (
  <div className="grv-portal-apps-n-sites">
    <Apps/>
    <Sites/>
    <div className="grv-footer-server-ver m-t-sm">
      <ServerVersion/>
    </div>
  </div>
)

export default AppsAndSites;
