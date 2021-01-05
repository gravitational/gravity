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