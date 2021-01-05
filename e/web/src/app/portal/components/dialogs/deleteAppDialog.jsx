import React from 'react';
import Button from 'oss-app/components/common/button';
import reactor from 'oss-app/reactor';
import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog
} from 'oss-app/components/dialogs/dialog';

import * as portalActions from './../../flux/apps/actions';
import portalGetters from './../../flux/apps/getters';

var DeleteAppDialog = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      deleteAttemp: portalGetters.deleteAppAttemp,
      appToDeleteId: portalGetters.appToDelete
    }
  },

  render: function() {
    let { appToDeleteId, deleteAttemp} = this.state;
    let { isProcessing } = deleteAttemp;

    if( !appToDeleteId ){
      return null;
    }

    return (
      <GrvDialog title="" className="grv-dialog-no-body grv-dialog-md">
        <GrvDialogHeader>
          <div className="grv-portal-dlg-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className="fa fa-exclamation-triangle fa-2x text-danger" aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">Are you sure?</h3>
              <div>
                <small>
                  You are about to delete <strong>{appToDeleteId}</strong> application from the Ops Center.
                </small>
              </div>
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogFooter>
          <Button
            className="btn-danger"
            onClick={()=> portalActions.deleteApp(appToDeleteId)}
            isProcessing={isProcessing}
            isDisabled={isProcessing}>
            Delete
          </Button>
          <Button
            isPrimary={false}
            className="btn btn-white"
            isDisabled={isProcessing}
            onClick={portalActions.closeAppConfirmDelete}>
            Close
          </Button>
        </GrvDialogFooter>
      </GrvDialog>
    );
  }
});

export default DeleteAppDialog;
