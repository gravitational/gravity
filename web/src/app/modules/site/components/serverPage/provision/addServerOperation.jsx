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
import reactor from 'app/reactor';
import * as actions from './../../../flux/servers/actions';
import getters from './../../../flux/servers/getters';
import $ from 'jQuery';
import Button from 'app/components/common/button';
import { ServerInstructions } from 'app/components/provision/items';
import AjaxPoller from 'app/components/dataProviders';
import ServerList from 'app/components/provision/serverList';
import opAgentGetters from 'app/flux/opAgent/getters';

const AddExistingServerOperation = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    let {opId} = this.props;
    return {
      serverCount: opAgentGetters.serverCountByOp(opId),
      model: getters.existingServerPendingOperation(opId),
      startOperationAttempt: getters.startOperationAttempt,
      deleteOperationAttempt: getters.deleteOperationAttempt
    }
  },

  onStart(){
    let $form = $(this.refs.form);
    $form.validate().settings.ignore = [];
    if($form.valid()){
      actions.startOperation();
    }
  },

  render() {
    let { startOperationAttempt, deleteOperationAttempt, serverCount} = this.state;

    let isDeleting = deleteOperationAttempt.isProcessing;
    let isCreating = startOperationAttempt.isProcessing;

    let isStartDisabled = isDeleting || serverCount === 0;
    let startBtnText = serverCount > 0 ? 'Start' : 'Waiting for servers...';

    return (
      <form ref="form">
        <AjaxPoller onFetch={actions.fetchAgentReport} />
        {this.renderFormContent()}
        <div className="row">
          <div className="col-sm-12 m-t-lg">
            <Button className="btn-primary grv-site-servers-btn-start"
              onClick={this.onStart}
              isProcessing={isCreating}
              isDisabled={isStartDisabled}>
              <span>{startBtnText}</span>
            </Button>
            <Button
              className="btn-default m-l grv-site-servers-btn-cancel"
              onClick={actions.cancelExpandOperation}
              isProcessing={isDeleting}
              isDisabled={isCreating}>
              <span>Cancel</span>
            </Button>
          </div>
        </div>
      </form>
    )
  },

  renderFormContent(){
    if(!this.state.model){
      return null;
    }

    let {opId} = this.props;
    let { profile, instructions } = this.state.model;
    let {description, title, value} = profile;

    return (
      <div className="row">
        <div className="col-sm-12">
          <h3 className="m-b-l">
            <span className="m-r-xs"> {title} </span>
            <small className="text-muted pull-right">{description}</small>
          </h3>
          <ServerInstructions text={instructions}/>
          <ServerList opId={opId} serverRole={value} />
        </div>
      </div>
    )
  }

});

export default AddExistingServerOperation;