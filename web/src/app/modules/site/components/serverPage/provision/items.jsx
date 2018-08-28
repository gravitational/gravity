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

import $ from 'jQuery';
import React from 'react';
import Button from 'app/components/common/button';
import {values} from 'lodash';
import RadioGroup from 'app/components/common/radioGroup.jsx';
import * as InputGroups from 'app/components/inputGroups';


/**
* Allows to select a node profile for onprem provisioners
*/
const ProfileSelector = React.createClass({
  onCancel(){
    if(this.props.onCancel){
      this.props.onCancel();
    }
  },

  onOk(){
    this.props.onOk();
  },

  render() {
    let {isProcessing} = this.props.attemp;
    let {value, profiles, onChange} = this.props;
    let isDisabled = !value || isProcessing;
    let options = values(profiles).map(item=> ({
      value: item.value,
      title: `${item.title} (${item.description})` }));

    return (
    <div>
      <div className="">
        <h3 className="m-b" style={{fontSize: '15px'}}>Select Profile</h3>
        <RadioGroup options={options} value={value} onChange={onChange}/>
      </div>
      <div className="row m-t-lg">
        <div className="col-sm-10">
          <Button
            className="btn-primary"
            onClick={ isDisabled ? ()=>{} : this.onOk}
            isProcessing={isProcessing}>
            Continue
          </Button>
          <Button
            className="btn-default m-l"
            isPrimary={false}
            onClick={this.onCancel}
            isDisabled={isProcessing}>
            Cancel
          </Button>
        </div>
      </div>
    </div>
    );
  }
});


/**
* Allows to select a provider keys (private and secret) profile for
* automatic provisioners
*/

class ProviderKeys extends React.Component {

  constructor(props) {
    super(props);
    this.accessKey = '';    
    this.secretKey = '';
    this.sessionToken = '';  
  }

  onAccessKeyChange = value => {    
    this.accessKey = value;    
  }

  onSecretKeyChange = value => {    
    this.secretKey = value;
  }

  onSessionKeyChange = value => {    
    this.sessionToken = value;  
  }  
  
  onCancel = () => {
    if(this.props.onCancel){
      this.props.onCancel();
    }
  }

  onOk = () => {
    if($(this.refs.form).valid()){
      this.props.onOk(this.accessKey, this.secretKey, this.sessionToken);
    }
  }
  
  render() {    
    let {isProcessing} = this.props.attemp;
    return (
    <div>
      <form ref="form">
        <div className="row">
          <InputGroups.AwsAccessKey className="col-sm-5" onChange={this.onAccessKeyChange} />
          <InputGroups.AwsSecretKey className="col-sm-6" onChange={this.onSecretKeyChange} />  
          <InputGroups.AwsSessionToken className="col-sm-12" onChange={this.onSessionKeyChange}/>                                
        </div>
        <div className="row m-t">
          <div className="col-sm-10">
            <Button
              className="btn-primary"
              onClick={this.onOk}
              isProcessing={isProcessing}>
              Continue
            </Button>
            <Button
              className="btn-default m-l"
              isPrimary={false}
              onClick={this.onCancel}
              isDisabled={isProcessing}>
              Cancel
            </Button>
          </div>
        </div>
      </form>
    </div>
    );
  }
}

export {
  ProviderKeys,
  ProfileSelector
}
