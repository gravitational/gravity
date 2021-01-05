import React from 'react';
import Button from 'oss-app/components/common/button';
import reactor from 'oss-app/reactor';
import {
  GrvDialogHeader,
  GrvDialogFooter,
  GrvDialog } from 'oss-app/components/dialogs/dialog';

import * as portalActions from './../../flux/sites/actions';
import portalGetters from './../../flux/sites/getters';

const DeleteAppDialog = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      unlinkAttemp: portalGetters.unlinkSiteAttemp,
      siteId: portalGetters.siteToUnlink
    }
  },

  render: function() {
    let { siteId, unlinkAttemp} = this.state;
    let { isProcessing } = unlinkAttemp;

    if( !siteId ){
      return null;
    }

    return (
      <GrvDialog title="" className="grv-dialog-no-body grv-dialog-md">
        <GrvDialogHeader>
          <div className="grv-portal-dlg-header">
            <div className="m-t-xs m-l-xs m-r-md">
              <i className="fa fa-plug fa-2x text-warning" aria-hidden="true"></i>
            </div>
            <div>
              <h3 className="m-b-xs">Are you sure?</h3>
              <div>
                <small>You are about to permanently disconnect <strong>{siteId}</strong> from the Ops Center. It will continue to work as usual in standalone mode.</small>
              </div>
            </div>
          </div>
        </GrvDialogHeader>
        <GrvDialogFooter>
          <Button
            className="btn-warning"
            onClick={()=> portalActions.unlinkSite(siteId)}
            isProcessing={isProcessing}>
            Remove
          </Button>
          <Button
            isPrimary={false}
            className="btn btn-white"
            isDisabled={isProcessing}
            onClick={portalActions.closeSiteConfirmUnlink}>
            Close
          </Button>
        </GrvDialogFooter>
      </GrvDialog>
    );
  }
});

export default DeleteAppDialog;