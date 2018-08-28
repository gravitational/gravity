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
import * as Alerts from 'app/components/common/alerts';
import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from 'app/components/dialogs/dialog';

const DeleteUserDialog = props => {
  let {
    userId,
    attemp,
    onContinue,
    onCancel
  } = props;

  if( !userId ){
    return null;
  }

  let { isProcessing, isFailed, message } = attemp;

  return (
    <GrvDialog title="" className="grv-dialog-no-body grv-dialog-sm grv-dialog-confirm grv-settings-dialog-with-errors">
      <GrvDialogHeader>
        <div className="grv-dialog-confirm-header">
          <div className="m-t-xs m-l-xs m-r-md">
            <i className="fa fa-exclamation-triangle fa-2x text-danger" aria-hidden="true"></i>
          </div>
          <div>
            <h3 className="m-b-xs">Are you sure?</h3>
            <div>
              <small>
                You are about to delete user <strong>{userId}</strong> from the Ops Center.
              </small>
            </div>
          </div>
        </div>
        { isFailed && <Alerts.Danger>{message} </Alerts.Danger> }
      </GrvDialogHeader>
      <GrvDialogFooter>
        <Button
          className="btn-danger m-r-sm"
          onClick={ () => onContinue(userId) }
          isProcessing={isProcessing}
          isDisabled={isProcessing}>
          Delete
        </Button>
        <Button
          isPrimary={false}
          className="btn btn-white"
          isDisabled={isProcessing}
          onClick={onCancel}>
          Close
        </Button>
      </GrvDialogFooter>
    </GrvDialog>
  );
}

export default DeleteUserDialog;
