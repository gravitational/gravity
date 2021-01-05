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