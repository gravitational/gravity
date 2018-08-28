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

import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from 'app/components/dialogs/dialog';

var UpdateAppConfirmDialog = React.createClass({

  onContinue(){
    let { onContinue, appId } = this.props;
    if(onContinue){
      onContinue(appId);
    }
  },

  render() {
    let { appId, onCancel, attemp } = this.props;
    let { isProcessing } = attemp;

    if( !appId ){
      return null;
    }

    let [, name, version] = appId.split('/');

    return (
      <GrvDialog title="" className="grv-dialog-no-body grv-dialog-confirm grv-site-dlg-update-app">
        <GrvDialogHeader>
          <div className="grv-dialog-confirm-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className="fa fa-exclamation-triangle fa-2x text-warning" aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">Version Upgrade</h3>
              <div>
                <small>You are about to update <strong>{name}</strong> to version <strong>{version}</strong>.</small>
                <div>
                  <small> Are you sure? </small>
                </div>
              </div>
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogFooter>
          <Button
            className="btn-warning"
            onClick={this.onContinue}
            isProcessing={isProcessing}>
            Update
          </Button>
          <Button
            isPrimary={false}
            className="btn btn-white"
            isDisabled={isProcessing}
            onClick={onCancel}>
            Cancel
          </Button>
        </GrvDialogFooter>
      </GrvDialog>
    );
  }
});

export default UpdateAppConfirmDialog;
