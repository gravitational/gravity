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

const ConfirmDialog = (props) => {  
  let { isVisible, onOk, onCancel } = props;

  if(!isVisible){
    return null;
  }

  return (
    <GrvDialog title="" className="grv-dialog-no-body grv-dialog-sm grv-dialog-confirm" >
      <GrvDialogHeader>
        <div className="grv-dialog-confirm-header">
          <div className="m-t-xs m-l-xs m-r-md">
            <i className="fa fa-exclamation-triangle fa-2x text-warning" aria-hidden="true"></i>
          </div>
          <div>
            <h3 className="m-b-xs">You have unsaved changes!</h3>
            <small>If you navigate away you will lose your unsaved changes.</small>
          </div>
        </div>
      </GrvDialogHeader>
      <GrvDialogFooter>
        <Button onClick={onOk} className="btn-warning">
          Disregard and continue
        </Button>
        <Button
          onClick={onCancel}
          isPrimary={false}
          className="btn btn-white">
          Close
        </Button>
      </GrvDialogFooter>
    </GrvDialog>
  )
}

export default ConfirmDialog;