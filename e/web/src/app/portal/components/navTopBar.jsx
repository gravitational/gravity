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
import UserLogo from 'oss-app/components/nav/userLogo';
import {CustomerLogo, GravitationalLogo} from 'oss-app/components/icons';
import cfg from 'oss-app/config'

const menuItems = [
  {
    to: cfg.routes.portalSettings,
    text: "Settings"
  }
];

const PortalNavTopBar = ({logoUri}) => {
  let headerText = cfg.getOpsCenterHeaderText();
  let $logo = null;
  if (logoUri) {
    $logo = (
      <CustomerLogo className="grv-portal-nav-customer-logo" imageUri={logoUri} />
    )
  } else {
    $logo = (
      <GravitationalLogo />
    )
  }

  /*
  if (!cfg.isDevCluster()) {
    menuItems.unshift({
      to: cfg.getSiteRoute(cfg.getLocalSiteId()),
      text: "Manage this cluster"
    })
  }
  */

  return (
    <div className="border-bottom">
      <div className="grv-portal-nav-top m-sm m-r">
        <div className="grv-portal-nav-top-item">
          <a href={cfg.routes.app} style={{
            fontSize: "1px"
          }}>
            {$logo}
          </a>
          <div className="no-margins">
            <span className="m-l-sm">{headerText}</span>
          </div>
        </div>
        <div className="grv-portal-nav-top-item grv-portal-nav-top-item-user">
          <UserLogo menuItems={menuItems}/>
        </div>
      </div>
    </div>
  )
}

export default PortalNavTopBar;