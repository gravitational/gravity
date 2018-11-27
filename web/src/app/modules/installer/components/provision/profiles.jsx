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
import { values } from 'lodash';
import * as actions from './../../flux/provision/actions';
import { Select } from 'app/components/common/dropDown';
import { ServerInstructions } from 'app/components/provision/items.jsx';
import AjaxPoller from 'app/components/dataProviders';
import ServerList from 'app/components/provision/serverList.jsx';
import WhatIs from 'app/components/whatis';

const Profiles = React.createClass({

  renderDescription({count, title, description }){
    return (
      <div>
        <div className="col-sm-6 col-xs-6">
          <div className="grv-installer-provision-node">
            <div className="grv-installer-provision-node-count" style={{textAlign: 'center'}}>
              <h2 className="no-margins">{count}</h2>
              <small>Nodes</small>
            </div>
            <div className="m-l grv-installer-provision-node-desc">
              <h3 className="no-margins">{title}</h3>
              <small className="text-muted">{description}</small>
            </div>
          </div>
        </div>
      </div>
    )
  },

  renderAwsItem(item, key){
    let {serverRole, instanceType, instanceTypes, instanceTypeFixed} = item;
    let $description = this.renderDescription(item);
    let $input = null;
    let reactKey = genProfileReactKey(key, serverRole);

    if(instanceTypes.length === 1 || instanceTypeFixed){
      $input = <div>{instanceType}</div>
    }else{
      $input = (
        <Select
          className="grv-installer-aws-instance-type"
          classRules="required"
          name={"awsInstance" + key}
          placeholder="Choose aws instance..."
          value={instanceType}
          onChange={ val => actions.setAwsInstanceType(serverRole, val) }
          options={instanceTypes}
        /> );
    }

    return (
      <div key={reactKey} className="grv-installer-provision-reqs-item">
        <div className="row">
          {$description}
          <div className="col-sm-4 col-xs-4 pull-right">
            <label>
              <span> Instance type </span>
              <WhatIs.AwsInstanceType placement="top"/>
            </label>
            {$input}
          </div>
        </div>
      </div>
    )
  },

  renderOnPremItem(item, key){
    let {opId} = this.props;
    let {instructions, serverRole} = item;
    let $description = this.renderDescription(item);
    let reactKey = genProfileReactKey(key, serverRole);
    return (
      <div key={reactKey} className="grv-installer-provision-reqs-item">
        <div className="row">
          {$description}
        </div>
        <div className="row">
          <div className="col-sm-12 col-sm-offset m-t">
            <ServerInstructions text={instructions}/>
          </div>
        </div>
        <ServerList opId={opId} serverRole={serverRole} />
      </div>
    )
  },

  render() {
    let {isOnPrem, profilesToProvision} = this.props;
    let profiles = values(profilesToProvision);
    let $reqItems = isOnPrem ? profiles.map(this.renderOnPremItem) :
      profiles.map(this.renderAwsItem);

    let $onPremAgentReportProvider = isOnPrem ? <AjaxPoller onFetch={actions.fetchAgentReport} /> : null;

    return (
      <form className="grv-installer-provision-reqs m-t-lg m-b">
        {$onPremAgentReportProvider}
        {$reqItems}
      </form>
    );
  }
})

function genProfileReactKey(index, role){
  return `${index}_${role}`
}

export default Profiles;