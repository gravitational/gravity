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
import Button from 'app/components/common/button';
import classnames from 'classnames';
import RemoteAssistantConfirmDialog from './../dialogs/remoteAssistantConfirmDialog';
import * as currentSiteActions from './../../flux/currentSite/actions';
import { ToolBar } from './items';

var RemoteAssistant = React.createClass({

  onEnable(){
    if(!this.props.enabled){
      currentSiteActions.openRemoteAccessDialog();
    }
  },

  onDisable(){
    if(this.props.enabled){
      currentSiteActions.openRemoteAccessDialog();
    }
  },

  render(){
    let { enabled } = this.props;
    let isOn = enabled === true;
    let isOff = !enabled;
    
    let btnONClass = classnames('btn-sm', {
      'btn-default': !isOn,
      'btn-primary': isOn
    });

    let btnOFFClass = classnames('btn-sm', {
      'btn-default': !isOff,
      'btn-primary': isOff
    });

    let tooltip = `remote assistance is ${ isOn ? "enabled" : "disabled"}`;

    return (
      <div className="grv-site-app-remote-assistance">
        <RemoteAssistantConfirmDialog />
        <ToolBar>
          <h3 className="grv-site-app-h3">
            <i className="fa fa-plug fa-lg m-r" aria-hidden="true"></i>
            <span title="Tooltip on left">Remote Assistance</span>
          </h3>
          <div title={tooltip} className="btn-group grv-site-app-remote-assistance-switch" data-toggle="buttons">
            <Button onClick={this.onEnable} className={btnONClass} isPrimary={isOn}> ON </Button>
            <Button onClick={this.onDisable} className={btnOFFClass} isPrimary={isOff}> OFF </Button>
          </div>
        </ToolBar>
        <div className="m-t-sm row">
          <div className="col-sm-12">
            <p>
              Remote access allows us to provide live assistance regarding your performance or operation of your cluster. If this is turned off, we will not be allowed access.
            </p>
          </div>
        </div>
      </div>
    )
  }
});

export default RemoteAssistant;