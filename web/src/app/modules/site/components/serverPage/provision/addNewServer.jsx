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
import {DropDown} from 'app/components/common/dropDown';
import reactor from 'app/reactor';
import {ProviderKeys} from './items.jsx';
import * as actions from './../../../flux/servers/actions';
import getters from './../../../flux/servers/getters';
import $ from 'jQuery';
import Button from 'app/components/common/button';
import { values } from 'lodash';
import { If } from 'app/components/common/helpers';

const AddNewServerOperation = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      model: getters.newServer,
      startOperationAttemp: getters.startOperationAttemp,
      deleteOperationAttemp: getters.deleteOperationAttemp,
      createOperationAttemp: getters.createOperationAttemp
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
    let {
      needKeys,
      selectedProfileKey,
      instanceType,
      instanceTypeOptions,
      profiles} = this.state.model;

    let {
      startOperationAttemp,
      createOperationAttemp,
      deleteOperationAttemp} = this.state;

    let isDeleting = deleteOperationAttemp.isProcessing;
    let isCreating = startOperationAttemp.isProcessing;
    let isProcessing = isDeleting || isCreating;

    let profileOptions = values(profiles).map(item=> ({
      value: item.value,
      label: item.title}));

    let {opId} = this.props;

    if(!opId && needKeys){
      return (
        <ProviderKeys
          attemp={createOperationAttemp}
          onOk={actions.createExpandOperation}
          onCancel={actions.cancelExpandOperation} />
      )
    }

    return (
      <form ref="form" className="grv-site-servers-provisioner-new">
        <div className="row">
          <div className="form-group  col-sm-3 grv-site-servers-provisioner-new-profile">
            <label>Profile</label>
            <DropDown
              classRules="required"
              name="profile"
              onChange={actions.setProfile}
              value={selectedProfileKey}
              options={profileOptions}
            />
          </div>
          <If isTrue={!!selectedProfileKey}>
            <div key="inst_type" className="form-group col-sm-3 grv-site-servers-provisioner-new-instance-type">
              <label>Instance Type</label>
              <DropDown
                classRules="required"
                name="instanceType"
                onChange={actions.setInstanceType}
                value={instanceType}
                options={instanceTypeOptions}/>
            </div>            
          </If>
        </div>
        <div className="row">
          <div className="col-sm-12 m-t">
            <Button className="btn-primary grv-site-servers-btn-start"
              onClick={this.onStart}
              isProcessing={isCreating}
              isDisabled={isProcessing}>
              <span>Start</span>
            </Button>
            <Button
              onClick={actions.cancelExpandOperation}
              isProcessing={isDeleting}
              isDisabled={isProcessing} className="btn-default m-l grv-site-servers-btn-cancel">
              <span>Cancel</span>
            </Button>
          </div>
        </div>
      </form>
    )
  }
});

export default AddNewServerOperation;