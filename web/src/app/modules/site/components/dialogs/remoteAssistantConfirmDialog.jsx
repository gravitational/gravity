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
import reactor from 'app/reactor';
import classnames from 'classnames';
import currentSiteGetters from './../../flux/currentSite/getters';
import * as currentSiteActions from './../../flux/currentSite/actions';

import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from  'app/components/dialogs/dialog';

const RemoteAccessDialog = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      model: currentSiteGetters.currentSiteRemoteAccess,
      attemp: currentSiteGetters.changeRemoteAccessAttemp
    }
  },

  onContinue(){
    let { enabled } = this.state.model;
    currentSiteActions.changeRemoteAccess(!enabled);
  },

  render: function() {
    let { isDialogOpen, enabled } = this.state.model;
    let { attemp } = this.state;
    let { isProcessing } = attemp;

    if(!isDialogOpen){
      return null;
    }
    
    let $title = null;
    let $description = null;
    let btnText = null;

    if(!enabled){
      btnText = 'Enable';
      $title = <span>Are you sure you want <strong>enable</strong> remote assistance? </span>
      $description = <span>Enabling remote assistance will allow vendor team to support your infrastructure.</span>
    }else{
      btnText = 'Disable';
      $title = <span>Are you sure you want <strong>disable</strong> remote assistance? </span>
      $description = <span>Disabling remote assistance will turn off remote access to your infrastructure for vendor support team.</span>
    }

    let iconClass = classnames('fa fa-2x', {
      'fa-question-circle-o fa-2x' : !enabled,
      'fa-exclamation-triangle fa-2x text-warning' : enabled
    });

    let dialogClass = classnames('grv-dialog-no-body grv-site-dlg-remote-access', {
      '--enabled' : enabled,
      '--disabled' : !enabled
    });

    let btnClass = enabled ? 'btn-danger' : 'btn-primary';

    return (
      <GrvDialog ref="dialog" className={dialogClass}>
        <GrvDialogHeader>
          <div className="grv-site-dlg-remote-access-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className={iconClass} aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">{$title}</h3>
              <small>{$description}</small>
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogFooter>
          <Button
            className={btnClass}
            isProcessing={isProcessing}
            isDisabled={isProcessing}
            onClick={this.onContinue}>
            {btnText}
          </Button>
          <Button
            onClick={currentSiteActions.closeRemoteAccessDialog}
            isPrimary={false}
            isDisabled={isProcessing}
            className="btn btn-white">
            Cancel
          </Button>
        </GrvDialogFooter>
      </GrvDialog>
    );
  }
});

export default RemoteAccessDialog;