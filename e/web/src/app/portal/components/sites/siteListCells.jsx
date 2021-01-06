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
import { displayDate } from 'oss-app/lib/dateUtils';
import { ProviderEnum } from 'oss-app/services/enums';
import { Cell } from 'oss-app/components/common/tables/table.jsx';

const StatusCell = ({ rowIndex, data, ...props }) => {
  let { state } = data[rowIndex];

  let $status = <span className="label label-default">Unknown</span>;

  if(state.isCreated){
    $status = <span className="label label-warning">Not Installed</span>;
  }

  if(state.isDeployed){
    $status = <span className="label label-primary">Deployed</span>;
  }

  if(state.isInstalling){
    $status = <span className="label label-warning">Installing</span>;
  }

  if(state.isUninstalling){
    $status = <span className="label label-warning">Uninstalling</span>;
  }

  if(state.isExpanding){
    $status = <span className="label label-warning">Expanding</span>;
  }

  if(state.isShrinking){
    $status = <span className="label label-warning">Shrinking</span>;
  }

  if(state.isFailed){
    $status = <span className="label label-danger">Failed</span>;
  }

  if(state.isDegraded){
    $status = <span className="label label-warning">Degraded</span>;
  }

  if(state.isOffline){
    $status = <span className="label label-default">Offline</span>;
  }

  if(state.isUpdating){
    $status = <span className="label label-warning">Updating</span>;
  }

  return (
    <Cell {...props}>
      {$status}
    </Cell>
  )
};

const DeployedByCell = ({ rowIndex, data, ...props }) => {
  let { createdBy, created } = data[rowIndex];
  let createdDisplayDate = displayDate(created);
  return (
    <Cell {...props}>
      <div style={{wordBreak: "break-all", minWidth: '120px'}}>
        <div>
          {createdDisplayDate}
        </div>
      <small className="text-muted">by: {createdBy}</small>
      </div>
    </Cell>
  )
};

const DeploymentCell = ({ rowIndex, data, ...props }) => {
  let { state, installUrl, siteUrl, domainName  } = data[rowIndex];
  let viewUrl = state.isInstalling ? installUrl : siteUrl;

  return (
    <Cell {...props}>
      <div style={{display: 'block', minWidth: '110px'}}>
        <a className="grv-portal-list-cell-deployment" href={viewUrl}>{domainName}</a>
      </div>
    </Cell>
  )
};

const AppNameCell = ({ rowIndex, data, ...props }) => {
  let {appDisplayName, appVersion} = data[rowIndex];
  return (
    <Cell {...props}>
      <div>
        <span>{appDisplayName}</span>
      </div>
      <small className="text-muted">ver. {appVersion}</small>
    </Cell>
  )
};

const ProviderCell = ({ rowIndex, data, ...props }) => {
  let { labels={}, provider, location } = data[rowIndex];
  let providerLabel;

  switch (provider) {
    case ProviderEnum.AWS:
      if(location){
        location = ` - ${location}`
      }

      providerLabel = `AWS${location}`;
      break;
    case ProviderEnum.ONPREM:
      providerLabel = 'On premise';
      break;
    default:
      providerLabel = 'unknown';
  }

  let $labels = Object.keys(labels).map((key, index) =>
    <small key={index} className="grv-portal-sites-tag">{key}:{labels[key]}</small> );

  return (
    <Cell {...props}>
      <span>{providerLabel}</span>
      { $labels.length > 0 ?
        <div className="text-muted" title="labels" style={{minWidth: '180px', wordBreak: 'break-all'}}>
          <i style={{fontSize: '10px'}} className="fa fa-tags m-r-xs" aria-hidden="true"></i>
          {$labels}
        </div> : null
      }
    </Cell>
  )
}

const ActionCell = ({ rowIndex, data, ...props }) => {
  let { onClickUnlink, onClickUninstall } = props;

  let {
    id,
    state,
    installUrl,
    siteUrl
  } = data[rowIndex];


  let viewUrl = state.isInstalling ? installUrl : siteUrl;
  let isOffLine = state.isOffline;

  return (
    <Cell {...props}>
      <div className="btn-group pull-right">
        <button type="button" className="btn btn-default btn-sm dropdown-toggle" data-toggle="dropdown" aria-haspopup="trufe" aria-expanded="false">
          <span className="m-r-xs">Actions</span>
          <span className="caret" />
        </button>
        <ul className="dropdown-menu dropdown-menu-right pull-right">

          { isOffLine ? null :
            <li>
              <a href={viewUrl}>
                <i className="fa fa-folder m-r-xs"></i>
                <span>View</span>
              </a>
            </li>
          }
          <li>
            <a onClick={ () => onClickUnlink(id) }>
              <i className="fa fa fa-plug"></i>
                <span className="m-r-xs"> Remove from Ops Center...  </span>
            </a>
          </li>
          { isOffLine ? null :
            <li>
              <a onClick={ () => onClickUninstall(id) }>
                <i className="fa fa-trash m-r-xs"></i>
                  <span>Delete...</span>
              </a>
            </li>
          }
        </ul>
      </div>

    </Cell>
  )
};

export {
  StatusCell,
  DeployedByCell,
  DeploymentCell,
  ProviderCell,
  AppNameCell,
  ActionCell
};
