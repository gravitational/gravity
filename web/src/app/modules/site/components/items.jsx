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
import cfg from 'app/config';
import reactor from 'app/reactor';
import classnames from 'classnames';
import currentSiteGetters from './../flux/currentSite/getters';
import { OpTypeEnum } from 'app/services/enums';
import { Link } from 'react-router';

const HistoryLinkLabel = ({ opType, siteId }) => {
  let url = cfg.getSiteHistoryRoute(siteId);
  let suffix = '';
  switch (opType) {
    case OpTypeEnum.OPERATION_EXPAND:
      suffix = 'adding a server...';
      break;
    case OpTypeEnum.OPERATION_SHRINK:
      suffix = 'removing a server...';
      break;
    case OpTypeEnum.OPERATION_UPDATE:
      suffix = 'updating the cluster...';
      break;
    default:
      break;  
  }
    
  return (
    <div> 
      <span>
        <i style={{ "lineHeight": "0"}} className="fa fa-cog fa-spin m-r-xs" aria-hidden="true" />
        <span>in progress: </span>
      </span>  
      <Link to={url}>              
        {suffix}
      </Link>
    </div>
  )  
}

var SiteNavBarHeader = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      currentSite: currentSiteGetters.currentSite()
    }
  },

  render(){
    let {isReady, isError, isProcessing } = this.state.currentSite.status2;

    let style = {
      color: '#a7b1c2'
    }

    let statusClass = classnames('grv-site-nav-header text-center', {
      '--error': isError,
      '--ready': isReady,
      '--processing': isProcessing
    });

    let iconClass = classnames('fa fa-5x', {
      'fa-globe': isReady,
      'fa-cog fa-spin': isProcessing,
      'fa-exclamation-triangle': isError
    });

    let text = 'Healthy';

    if(isError){
      text = 'With Issues';
    }

    if(isProcessing){
      text = 'Processing...';
    }

    return (
      <div className={statusClass}>
        <div>
          <a href={cfg.routes.app}>
            <i className={iconClass}></i>
          </a>
        </div>
        <div style={style}>
          <div>System Status</div>
          <div style={{color: "#ffffff"}}>{text}</div>
        </div>
      </div>
    )
  }
})

export {
  HistoryLinkLabel,
  SiteNavBarHeader
}