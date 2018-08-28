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
import {Separator} from './../items.jsx';
import {Select} from 'app/components/common/dropDown';
import {NewOrExistingServers} from './items.jsx';
import getters from './../../flux/newApp/getters';
import * as actions from './../../flux/newApp/actions';
import WhatIs from 'app/components/whatis';
import connect from 'app/lib/connect';
import * as InputGroups from 'app/components/inputGroups';

const KeyPairName = ({options=[], value})=> {
  return (
    <div>
      <label>
        <span>Select your key pair </span>
        <WhatIs.AwsPairs placement="top"/>
      </label>
      <Select
        className="grv-installer-aws-key-pair"
        classRules="required"
        name="keyPairDropDown"
        searchable={false}
        clearable={false}
        value={value}
        onChange={actions.setKeyPairName}
        options={options}/>
    </div>
  )
}

class AwsProvider extends React.Component {
  
  onAccessKeyChange = value => {    
    this.accessKey = value;
    this.onKeysChange();
  }

  onSecretKeyChange = value => {    
    this.secretKey = value;
    this.onKeysChange();
  }
  
  onSessionKeyChange = value => {    
    this.sessionToken = value;
    this.onKeysChange();
  }

  onKeysChange(){    
    actions.setProviderKeys(this.accessKey, this.secretKey, this.sessionToken);
  }
    
  render() {
    let {
      useExisting,
      keysVerified,
      regions,
      selectedRegion,
      selectedVpc,
      selectedKeyPairName,
      regionKeyPairNames,
      regionVpcs} = this.props.model;

    return (
      <div>
        <form ref="form" className="m-t">
          <div className="row">
            <InputGroups.AwsAccessKey className="col-sm-6" onChange={this.onAccessKeyChange} />
            <InputGroups.AwsSecretKey className="col-sm-6" onChange={this.onSecretKeyChange} />  
            <InputGroups.AwsSessionToken className="col-sm-12" onChange={this.onSessionKeyChange}/>                      
        </div>
        {
          keysVerified ?
            <div>
              <div className="row">
                <div className="form-group  col-sm-6">
                  <label>
                    <span>Select your server region </span>
                    <WhatIs.AwsRegion placement="top"/>
                  </label>
                  <Select
                    className="grv-installer-aws-region"
                    classRules="required"
                    name="region"
                    searchable={false}
                    clearable={false}
                    value={selectedRegion}
                    onChange={actions.setAwsRegion}
                    options={regions}
                  />
                </div>
               { selectedRegion ? (
                 <div>
                   <div className="form-group col-sm-6">
                     <KeyPairName options={regionKeyPairNames} value={selectedKeyPairName}/>
                   </div>
                   <div className="form-group col-sm-offset-6 col-sm-6">
                      <label>
                        <span>Select your VPC </span>
                        <WhatIs.AwsVPC placement="top"/>
                      </label>
                      <Select
                        className="grv-installer-aws-vpc"
                        classRules="required"
                        name="vpc"
                        searchable={false}
                        clearable={false}
                        value={selectedVpc}
                        onChange={actions.setAwsVpc}
                        options={regionVpcs} />
                    </div>
                  </div> ) : null
                }
              </div>
              <Separator/>
              <div className="row">
                  <div className="col-sm-6">
                    <NewOrExistingServers useExisting={useExisting} onChange={actions.useNewServers}/>
                  </div>
              </div>
            </div>
            : null
        }
      </form>
    </div>
    );
  }
}

function mapStateToProps() {
  return {
    model: getters.awsProvider,
    verifyKeysAttemp: getters.verifyKeysAttemp
  }  
}

export default connect(mapStateToProps)(AwsProvider);

