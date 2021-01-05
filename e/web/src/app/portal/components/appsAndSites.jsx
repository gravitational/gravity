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
