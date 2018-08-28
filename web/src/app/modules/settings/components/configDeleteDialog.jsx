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

import React, { PropTypes } from 'react';
import Button from 'app/components/common/button';
import { ResourceEnum } from 'app/services/enums'
import * as Alerts from 'app/components/common/alerts';
import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from 'app/components/dialogs/dialog';

import connect from 'app/lib/connect';
import getters from './../flux/getters';
import { closeDeleteDialog } from './../flux/actions';

const getResourceKind = kind => {
  if(kind === ResourceEnum.OIDC || kind === ResourceEnum.SAML){
    return 'auth.connector'
  }

  if(kind === ResourceEnum.ROLE){
    return 'role'
  }

  if(kind === ResourceEnum.TRUSTED_CLUSTER){
    return 'trusted cluster'
  }

  if(kind === ResourceEnum.LOG_FWRD){
    return 'log forwarder'
  }

  return 'resource';
}

const ConfigDeleteDialog = props => {
  const { store, attempt, onContinue } = props;
  const { isProcessing, isFailed, message } = attempt;
  const resItem = store.getResourceToDelete();

  if( !resItem ){
    return null;
  }

  const id = resItem.getName();
  const kindText = getResourceKind(resItem.getKind());
  const messagePrefix = `You are about to delete ${kindText} `;

  return (
    <GrvDialog title="" className="grv-dialog-no-body grv-dialog-sm grv-dialog-confirm grv-settings-dlg-delete">
      <GrvDialogHeader>
        <div className="grv-dialog-confirm-header">
          <div className="m-t-xs m-l-xs m-r-md">
            <i className="fa fa-exclamation-triangle fa-2x text-danger" aria-hidden="true"></i>
          </div>
          <div>
            <h3 className="m-b-xs">Are you sure?</h3>
            <div>
              <small>
                {messagePrefix} <strong>{id}</strong>.
              </small>
            </div>
          </div>
        </div>
        { isFailed && <Alerts.Danger>{message} </Alerts.Danger> }
      </GrvDialogHeader>
      <GrvDialogFooter>
        <Button
          className="btn-danger"
          onClick={ () => onContinue(resItem) }
          isProcessing={isProcessing}
          isDisabled={isProcessing}>
          Delete
        </Button>
        <Button
          isPrimary={false}
          className="btn-white"
          isDisabled={isProcessing}
          onClick={closeDeleteDialog}>
          Cancel
        </Button>
      </GrvDialogFooter>
    </GrvDialog>
  );
}

ConfigDeleteDialog.propTypes = {
  store: PropTypes.object.isRequired,
  attempt: PropTypes.object.isRequired
};


function mapStateToProps() {
  return {
    store: getters.dialogStore,
    attempt: getters.deleteConfigAttempt
  }
}

export default connect(mapStateToProps)(ConfigDeleteDialog);

