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
import { ProviderEnum } from 'app/services/enums';
import UserLogo from 'app/components/nav/userLogo';
import reactor from 'app/reactor';
import currentSiteGetters from './../flux/currentSite/getters';
import classnames from 'classnames';
import cfg from 'app/config';
import { Link } from 'react-router';

const StatusIndicator = ({ isReady, isError, isProcessing, siteId }) => {
  let statusClass = classnames(`grv-site-nav-top-indicator border-left p-w-sm`, {
    '--error': isError,
    '--ready': isReady,
    '--processing': isProcessing
  });

  let iconClass = classnames('fa m-t-xs fa-2x m-r-sm', {
    'fa-exclamation-triangle': isError || isProcessing,
    'fa-check': isReady
    
  })

  let $content = 'Healthy';

  if(isError){
    $content = 'With Issues';
  }

  if (isProcessing) {
    let url = cfg.getSiteHistoryRoute(siteId);
    $content = (
      <Link to={url}> In progress... </Link>
    );
  }

  return (
    <div className={statusClass}>
      <div>
        <i className={iconClass}/>
      </div>  
      <div>
        <strong className="text-bold">System Status</strong>
        <div>{$content}</div>
      </div>
    </div>
  )
}

const LocationIndicator = ({ provider, location }) => {
  let providerLabel;

  switch (provider) {
    case ProviderEnum.AWS:
      if(location){
        location = ` - ${location}`
      }

      providerLabel = `AWS${location}`;
      break;
    case ProviderEnum.ONPREM:
      providerLabel = 'On premises';
      break;
    default:
      providerLabel = 'unknown';
  }

  return (
    <div className="grv-site-nav-top-indicator border-left p-w-sm">
      <div>
        <strong className="text-bold">Location</strong>
        <div>{providerLabel}</div>
      </div>
    </div>
  )
}

var SiteNavTopBar = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      siteModel: currentSiteGetters.currentSite()
    }
  },

  render(){
    let { appInfo, status2,  provider, location, id } = this.state.siteModel;
    let { displayName } = appInfo;       
    let menuItems = [];

    // do not show settings menu if connected via ops center
    if(!cfg.isRemoteAccess(id)){
      menuItems = [{        
        to: cfg.getSiteSettingsRoute(id),
        text: "Settings"
      } ]
    }
    
    return (
      <div className="row border-bottom">
        <div className="col-sm-12">
        <div className="grv-site-nav-top m-sm m-l m-r">
          <div className="grv-site-nav-top-item grv-site-nav-top-item-app">
            <div className="m-r">
              <h3 className="no-margins">
                {displayName}
              </h3>
            </div>
            <StatusIndicator {...status2} siteId={id}/>
            <LocationIndicator provider={provider} location={location}/>
          </div>
          <div className="grv-site-nav-top-item grv-site-nav-top-item-user">
            <UserLogo menuItems={menuItems}/>            
          </div>
          </div>
        </div>
      </div>
    );
  }
});

export default SiteNavTopBar;
