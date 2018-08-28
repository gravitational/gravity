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
import classnames from 'classnames';
import reactor from 'app/reactor';
import { ProviderEnum } from 'app/services/enums';
import { If } from 'app/components/common/helpers';
import AddNewServer from './addNewServer';
import AddExistingServer from './addExistingServer';
import * as actions from './../../../flux/servers/actions';
import getters from './../../../flux/servers/getters';

import { HistoryLinkLabel } from './../../items';

const ServerProvisioner = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      model: getters.serverProvision,
      createOperationAttemp: getters.createOperationAttemp
    }
  },

  renderButtons() {
    let { createOperationAttemp, model } = this.state;
    let { isProcessing } = createOperationAttemp;
    let {
      provider,
      isNewServer,
      isExistingServer } = model;

    let isAnyActive = isNewServer || isExistingServer;

    let addNewBtnClass = classnames('btn btn-sm grv-site-servers-provisioner-add-new', {
      'btn-primary active disabled': isNewServer,
      'disabled': isExistingServer || isProcessing,
      'btn-primary': !isAnyActive,
      'hidden': provider === ProviderEnum.ONPREM
    });

    let addExistingBtnClass = classnames('btn btn-sm m-l-sm grv-site-servers-provisioner-add-existing', {
      'btn-primary active disabled': isExistingServer,
      'disabled': isNewServer || isProcessing,
      'btn-primary': !isAnyActive
    });

    return (
      <div className="grv-site-servers-provisioner-header-controls">
        <a type="button"
            onClick={actions.initWithNewServer}
            className={addNewBtnClass}>
          Provision New
        </a>
        <a type="button"
            onClick={()=>actions.initWithExistingServer()}
            className={addExistingBtnClass}>
          Add Existing
        </a>
      </div>
    )
  },

  render() {
    let {
      siteId,
      inProgressOpId,
      inProgressOpType,
      initiatedOpId,
      isNewServer,
      isExistingServer } = this.state.model;

    let $headerContent = inProgressOpId ? (
      <HistoryLinkLabel opType={inProgressOpType} siteId={siteId} />
    ) : this.renderButtons();

    return (
      <div className="grv-site-servers-provisioner">
        <div className="grv-site-servers-provisioner-header m-b-sm">
          <div>
            <h3 className="grv-site-header-size no-margins">
              <span>Servers</span>
            </h3>
          </div>
          {$headerContent}
        </div>
        <If isTrue={isNewServer}>
          <MakeBox>
            <AddNewServer opId={initiatedOpId}/>
          </MakeBox>
        </If>
        <If isTrue={isExistingServer}>
          <MakeBox>
            <AddExistingServer opId={initiatedOpId}/>
          </MakeBox>
        </If>
      </div>
    )
  }
});

const MakeBox = ({ children }) => (
  <div className="ibox m-b-md grv-site-servers-provisioner-content">
    <div className="ibox-content">
      {children}
  </div>
</div>
)

export default ServerProvisioner;