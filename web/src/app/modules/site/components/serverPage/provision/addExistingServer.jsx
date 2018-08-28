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
import {ProfileSelector} from './items';
import AddExistingServerOperation from './addExistingServerOperation';
import getters from './../../../flux/servers/getters';
import * as actions from './../../../flux/servers/actions';
import $ from 'jQuery';

var AddExistingServer = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      model: getters.existingServer,
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
    let { createOperationAttemp} = this.state;
    let { selectedProfileKey, profiles } = this.state.model;
    let {opId} = this.props;

    if(!opId){
      return (
        <ProfileSelector
          profiles={profiles}
          value={selectedProfileKey}
          onChange={actions.setProfile}
          attemp={createOperationAttemp}
          onOk={actions.createExpandOperation}
          onCancel={actions.cancelExpandOperation} />
      )
    }

    return (<AddExistingServerOperation opId={opId}/>);
  }
});

export default AddExistingServer;
