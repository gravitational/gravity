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

import React from 'react';
import UserLogo from 'app/components/nav/userLogo';
import { GravitationalLogo, CustomerLogo } from 'app/components/icons';

const SettingsNavTopBar = ({goBackUrl, goBackLabel, customerLogoUri}) => {
  let $logo = null;

  if(customerLogoUri){
    $logo = (
      <CustomerLogo
        className="grv-settings-nav-customer-logo"
        imageUri={customerLogoUri}
      />
    )
  }else{
    $logo = <GravitationalLogo/>
  }

  return (
    <div className="border-bottom">
        <div className="grv-settings-nav-top m-sm m-r">
          <div className="grv-settings-nav-top-item">
            <a href={goBackUrl} style={{fontSize: "1px"}}>
              {$logo}
            </a>
            <ol className="breadcrumb m-l-sm">
              <li><a href={goBackUrl}>{goBackLabel}</a></li>
              <li className="active"><a>Settings</a></li>
            </ol>
          </div>
          <div className="grv-settings-nav-top-item grv-settings-nav-top-item-user">
            <UserLogo />
          </div>
        </div>
    </div>
  )
}

export default SettingsNavTopBar;